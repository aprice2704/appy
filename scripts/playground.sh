#!/bin/bash
# :: product: FDM/NS
# :: description: Interactive playground harness for testing Appy manually.
# :: filename: scripts/playground.sh

set -e

REPO_ROOT=$(cd "$(dirname "$0")/.." && pwd)
PLAYGROUND_DIR="$REPO_ROOT/testdata/playground"
FIXTURES_DIR="$REPO_ROOT/testdata/fixtures"

echo "🧹 Cleaning old playground..."
rm -rf "$PLAYGROUND_DIR"
mkdir -p "$PLAYGROUND_DIR"

if [ -d "$FIXTURES_DIR" ]; then
    echo "📦 Seeding playground from fixtures..."
    cp -r "$FIXTURES_DIR"/* "$PLAYGROUND_DIR"/
else
    echo "🌱 Creating default fixture..."
    mkdir -p "$FIXTURES_DIR"
    cat << 'EOF' > "$FIXTURES_DIR/sample.go"
package main

func Old() {
	println("This is the old function.")
}
EOF
    cp "$FIXTURES_DIR/sample.go" "$PLAYGROUND_DIR"/
fi

echo "🚀 Building Appy..."
cd "$REPO_ROOT"
go install .

echo "🔥 Launching Appy within playground sandbox..."
cd "$PLAYGROUND_DIR"

if [ $# -eq 0 ]; then
    appy -port 8086
else
    appy "$@"
fi