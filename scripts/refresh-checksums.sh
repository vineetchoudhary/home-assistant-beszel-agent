#!/usr/bin/env bash
#
# Rewrite every app's beszel_sha256 from the checksums file published with an
# upstream Beszel release.
#
# The Dockerfiles verify the tarball they download against these digests and
# fail the build on a mismatch, so the digests have to move whenever
# beszel_version does. bump-version.sh and the publish workflow both call this
# for exactly that reason; run it by hand only when fixing something up.
#
#   ./scripts/refresh-checksums.sh 0.19.0             # refresh every app
#   ./scripts/refresh-checksums.sh 0.19.0 --dry-run   # report, write nothing
#
# Which assets an app installs is declared by the asset column already in its
# beszel_sha256 file. Only the digests are rewritten. If upstream renames an
# asset, edit that column by hand once and this script follows it afterwards.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}" || exit 1

BESZEL_VERSION=""
DRY_RUN=0

usage() { sed -n '2,18p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }

while [ $# -gt 0 ]; do
    case "$1" in
        -n|--dry-run) DRY_RUN=1 ;;
        -h|--help)    usage; exit 0 ;;
        -*)           echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
        *)            if [ -n "${BESZEL_VERSION}" ]; then
                          echo "Unexpected argument: $1" >&2; usage >&2; exit 2
                      fi
                      BESZEL_VERSION="$1" ;;
    esac
    shift
done

if [ -t 1 ]; then
    C_RED=$'\033[31m'; C_GRN=$'\033[32m'
    C_BLU=$'\033[34m'; C_DIM=$'\033[2m';  C_OFF=$'\033[0m'
else
    C_RED=""; C_GRN=""; C_BLU=""; C_DIM=""; C_OFF=""
fi
pass()  { printf '  %sPASS%s  %s\n' "${C_GRN}" "${C_OFF}" "$*"; }
info()  { printf '  %s%s%s\n' "${C_DIM}" "$*" "${C_OFF}"; }
head1() { printf '\n%s==> %s%s\n' "${C_BLU}" "$*" "${C_OFF}"; }
die()   { printf '  %sERROR%s %s\n' "${C_RED}" "${C_OFF}" "$*" >&2; exit 1; }

[ -n "${BESZEL_VERSION}" ] || { echo "Missing version." >&2; usage >&2; exit 2; }
[[ "${BESZEL_VERSION}" =~ ^[0-9]+(\.[0-9]+)*$ ]] || die "Not a version number: ${BESZEL_VERSION}"

APPS=""
while IFS= read -r dir; do
    [ -n "${dir}" ] || continue
    APPS="${APPS} ${dir}"
done <<EOD
$(find . -maxdepth 2 -name beszel_sha256 -path './beszel_*' -exec dirname {} \; | sed 's|^\./||' | sort)
EOD
[ -n "${APPS## }" ] || die "No app directories with a beszel_sha256 found under ${REPO_ROOT}"

CHECKSUMS_URL="https://github.com/henrygd/beszel/releases/download/v${BESZEL_VERSION}/beszel_${BESZEL_VERSION}_checksums.txt"

head1 "Fetching checksums for Beszel ${BESZEL_VERSION}"
info "${CHECKSUMS_URL}"

CHECKSUMS="$(mktemp)" || die "Could not create a temporary file"
trap 'rm -f "${CHECKSUMS}"' EXIT

curl --fail --silent --show-error --location --retry 3 --retry-delay 2 \
    -o "${CHECKSUMS}" "${CHECKSUMS_URL}" \
    || die "Could not download the checksums file. Does release v${BESZEL_VERSION} exist?"

# A truncated or HTML error body would otherwise sail through and produce a
# file full of digests that match nothing.
grep -qE '^[0-9a-f]{64}  ' "${CHECKSUMS}" \
    || die "Downloaded checksums file does not look like sha256sum output"

head1 "Files"
CHANGED=0
for app in ${APPS}; do
    file="${app}/beszel_sha256"

    assets="$(awk '!/^#/ && NF == 2 { print $2 }' "${file}")"
    [ -n "${assets}" ] || die "${app}: beszel_sha256 declares no assets"

    body=""
    changes=""
    for asset in ${assets}; do
        line="$(awk -v a="${asset}" '$2 == a { print; exit }' "${CHECKSUMS}")"
        [ -n "${line}" ] \
            || die "${app}: release v${BESZEL_VERSION} publishes no asset named ${asset}"

        new_sha="${line%% *}"
        old_sha="$(awk -v a="${asset}" '$2 == a { print $1; exit }' "${file}")"
        [ "${new_sha}" != "${old_sha}" ] && changes="${changes} ${asset}"

        body="${body}${new_sha}  ${asset}"$'\n'
    done

    if [ "${DRY_RUN}" -eq 0 ]; then
        # Keep the comment header the file already carries, and write through
        # the existing file so its inode and permissions survive.
        { awk '/^#/ || NF == 0' "${file}"; printf '%s' "${body}"; } > "${file}.new" \
            || die "${app}: could not stage beszel_sha256"
        cat "${file}.new" > "${file}" || die "${app}: could not write beszel_sha256"
        rm -f "${file}.new"
    fi

    if [ -n "${changes## }" ]; then
        CHANGED=$((CHANGED + 1))
        if [ "${DRY_RUN}" -eq 1 ]; then info "${app}: would update${changes}"
        else pass "${app}: updated${changes}"; fi
    else
        if [ "${DRY_RUN}" -eq 1 ]; then info "${app}: already current"
        else pass "${app}: already current"; fi
    fi
done

head1 "Done"
info "${CHANGED} of $(set -- ${APPS}; echo $#) apps changed"
