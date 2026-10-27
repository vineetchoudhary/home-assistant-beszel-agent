package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func resolveMQTT(ctx context.Context, c Config) (Config, error) {
	if c.MQTTSource == "manual" {
		return c, nil
	}
	token := os.Getenv("SUPERVISOR_TOKEN")
	if token == "" {
		return c, errors.New("Supervisor MQTT service is unavailable; configure mqtt_source: manual or install the broker app")
	}
	base := os.Getenv("BESZEL_BRIDGE_SUPERVISOR_URL")
	if base == "" {
		base = "http://supervisor"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/services/mqtt", nil)
	if err != nil {
		return c, errors.New("cannot construct Supervisor MQTT service request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Supervisor service redirect refused") }}
	response, err := client.Do(req)
	if err != nil {
		return c, errors.New("cannot reach Supervisor MQTT service; check broker setup")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return c, fmt.Errorf("Supervisor MQTT service returned HTTP %d; check broker setup", response.StatusCode)
	}
	var result struct {
		Result string `json:"result"`
		Data   struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			SSL      bool   `json:"ssl"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result) != nil || result.Result != "ok" || result.Data.Host == "" || result.Data.Port < 1 || result.Data.Port > 65535 {
		return c, errors.New("Supervisor MQTT service returned incomplete connection settings")
	}
	c.MQTTHost = result.Data.Host
	c.MQTTPort = result.Data.Port
	c.MQTTUsername = result.Data.Username
	c.MQTTPassword = result.Data.Password
	c.MQTTTLS = result.Data.SSL
	return c, nil
}
func brokerAddress(c Config) string {
	scheme := "tcp"
	if c.MQTTTLS {
		scheme = "ssl"
	}
	return scheme + "://" + net.JoinHostPort(c.MQTTHost, strconv.Itoa(c.MQTTPort))
}

type mqttPublisher struct{ client mqtt.Client }

func (p mqttPublisher) publish(topic string, payload any, retain bool) error {
	if !p.client.IsConnectionOpen() {
		return errors.New("MQTT broker is disconnected")
	}
	token := p.client.Publish(topic, 1, retain, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return errors.New("MQTT publish timed out")
	}
	if token.Error() != nil {
		return errors.New("MQTT publication failed")
	}
	return nil
}

func connectMQTT(c Config, s *State, connected chan<- bool) (mqtt.Client, error) {
	tlsSettings, err := tlsConfig(c.MQTTCAFile)
	if err != nil {
		return nil, err
	}
	options := mqtt.NewClientOptions().AddBroker(brokerAddress(c)).SetClientID("ha_"+s.Namespace).
		SetUsername(c.MQTTUsername).SetPassword(c.MQTTPassword).SetConnectTimeout(10*time.Second).
		SetWriteTimeout(5*time.Second).SetKeepAlive(30*time.Second).SetPingTimeout(10*time.Second).
		SetAutoReconnect(true).SetMaxReconnectInterval(60*time.Second).SetCleanSession(true).
		SetWill(s.root()+"/availability", "offline", 1, true).SetTLSConfig(tlsSettings)
	options.SetOnConnectHandler(func(client mqtt.Client) {
		// Subscribe before the main loop republishes discovery and current states.
		client.Subscribe(c.HAStatusTopic, 1, func(_ mqtt.Client, message mqtt.Message) {
			if string(message.Payload()) == "online" {
				select {
				case connected <- true:
				default:
				}
			}
		})
		select {
		case connected <- true:
		default:
		}
	})
	options.SetConnectionLostHandler(func(_ mqtt.Client, _ error) {
		select {
		case connected <- false:
		default:
		}
	})
	client := mqtt.NewClient(options)
	token := client.Connect()
	if !token.WaitTimeout(12 * time.Second) {
		client.Disconnect(0)
		return nil, errors.New("MQTT connection timed out")
	}
	if token.Error() != nil {
		return nil, errors.New("MQTT connection failed; check broker address, credentials, and TLS")
	}
	return client, nil
}
