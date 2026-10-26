#!/bin/sh
# Builds the two helper jars. Needs JDK 17+ and copies of ZombieBuddy.jar.
# Usage: build.sh <ZombieBuddy.jar> <output mod version dir, e.g. KnoxLink/42>
set -e
ZB="$1"; OUT="$2"; SRC="$(cd "$(dirname "$0")" && pwd)/java"
T=$(mktemp -d)
mkdir -p "$T/client" "$T/agent" "$OUT/media/java/client" "$OUT/media/java/agent"
javac --release 17 -nowarn -cp "$ZB" -d "$T/client" $(find "$SRC/client" -name '*.java')
rm -rf "$T/client/se"   # compile-time stub of the game's LuaMethod annotation
jar cf "$OUT/media/java/client/KnoxLink.jar" -C "$T/client" .
javac --release 17 -nowarn -cp "$ZB" -d "$T/agent" $(find "$SRC/agent" -name '*.java')
jar cfm "$OUT/media/java/agent/KnoxLinkAgent.jar" "$SRC/agent/MANIFEST.MF" -C "$T/agent" .
rm -rf "$T"
