#!/usr/bin/env bash
# refs.sh: clone the reference implementations read-only into third_party/refs.
# Existing reference checkouts are reset to the fetched commit.
# See docs/research.md sections 3, 4 and 5 for the reference catalogue and
# docs/research/decisions/0003-reference-projects-and-dependency-policy.md for
# the licence positions. Repositories marked "read only" carry no licence or
# a GPL licence: read them for protocol knowledge, never copy from them.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/third_party/refs"
mkdir -p "$DEST"
# A go.mod here makes every clone a foreign module, so ./... in the library
# never descends into a reference that ships without its own go.mod.
printf 'module third_party.invalid/refs\n\ngo 1.27.0\n' > "$DEST/go.mod"
REPOS=(
  fleetdm/fleet
  Malcolm/local-mdm
  mjoliver/Windows-MDM
  marcosd4h/mdm
  marcosd4h/MDMatador
  oscartbeaumont/windows_mdm
  mattrax/Mattrax
  PhenixH/Mattrax
  dejiblue/windows-mdm
  AmberWolfCyber/NachoMDM
  wso2/carbon-device-mgt-plugins
  entgra/device-mgt-plugins
  deploymenttheory/go-sdk-windowscsp
  deploymenttheory/go-bindings-win32
  deploymenttheory/go-bindings-winrt
  okieselbach/SyncMLViewer
  Sleepw4lker/TameMyCerts.WSTEP
  mattrax/xml
  remdev/go-activesync
  oniestel/go-wns
  secureworks/pytune
  Gerenios/AADInternals
  getprimo/csp-builder
  smallstep/pkcs7
  smallstep/scep
  google/go-attestation
  hotnops/Imitune
  kapytein/syncml
)
# Read-only (no licence or GPL): mjoliver/Windows-MDM, AmberWolfCyber/NachoMDM,
# hotnops/Imitune (GPL-3.0), kapytein/syncml (GPL-3.0). Mattrax's current
# branch is source-available, not open source.
for repo in "${REPOS[@]}"; do
  name="${repo##*/}"
  if [ -d "$DEST/$name/.git" ]; then
    echo "updating $repo"; git -C "$DEST/$name" fetch -q --depth=1 origin && git -C "$DEST/$name" reset -q --hard FETCH_HEAD
  else
    echo "cloning $repo"; git clone -q --depth=1 "https://github.com/$repo" "$DEST/$name"
  fi
  printf '%s %s\n' "$repo" "$(git -C "$DEST/$name" rev-parse HEAD)"
done > "$DEST/COMMITS.txt"
cat "$DEST/COMMITS.txt"
