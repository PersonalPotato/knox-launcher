#!/bin/sh
# Copies the working source into this repo. Run from the repo root:
#   tools/sync.sh /path/to/project
set -e
SRC="${1:?usage: tools/sync.sh <project folder>}"
rsync -a --delete --exclude ezhost --exclude devdata --exclude '*.exe' "$SRC/app/src/" launcher/
rsync -a --delete --exclude '*.jar' "$SRC/KnoxLink/" mod/KnoxLink/
rsync -a --delete "$SRC/src/" mod/src/
rsync -a --delete "$SRC/community-server/" server/
git status --short
