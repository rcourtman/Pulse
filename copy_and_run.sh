cd /
if [ -d /tmp/pulse-copy ]; then
 rm -rf /tmp/pulse-copy
fi
mkdir -p /tmp/pulse-copy
cp -R /opt/pulse /tmp/pulse-copy/repo
cd /tmp/pulse-copy/repo
GOCACHE=/tmp/go-cache HOME=/tmp/pulse-copy/repo go test ./...
