#!/bin/bash
# :: product: FDM/NS
# :: description: Hot-reloading wrapper for the Appy server.
# :: filename: run_appy.sh

echo "🚀 Starting FDM Appy in hot-reload mode..."
while true; do
    appy "$@"
    EXIT_CODE=$?
    if [ $EXIT_CODE -ne 42 ]; then
        echo "Appy exited normally or with a fatal error (Code $EXIT_CODE). Stopping."
        break
    fi
    echo "🔄 Appy binary updated! Hot-restarting..."
    sleep 1
done