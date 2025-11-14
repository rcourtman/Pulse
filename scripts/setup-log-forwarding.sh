#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "Run as root" >&2
  exit 1
fi

REMOTE_HOST=${REMOTE_HOST:-logs.pulse.example}
REMOTE_PORT=${REMOTE_PORT:-6514}
CERT_DIR=${CERT_DIR:-/etc/pulse/log-forwarding}
CA_CERT=${CA_CERT:-$CERT_DIR/ca.crt}
CLIENT_CERT=${CLIENT_CERT:-$CERT_DIR/client.crt}
CLIENT_KEY=${CLIENT_KEY:-$CERT_DIR/client.key}

install -d -m 0750 "$CERT_DIR"

CONF_PATH=/etc/rsyslog.d/pulse-sensor-proxy.conf
cat <<EOF >"$CONF_PATH"
module(load="imfile" PollingInterval="5")

input(type="imfile"
      File="/var/log/pulse/sensor-proxy/audit.log"
      Tag="pulse.audit"
      Facility="local4"
      Severity="notice"
      PersistStateInterval="100"
      addMetadata="on")

input(type="imfile"
      File="/var/log/pulse/sensor-proxy/proxy.log"
      Tag="pulse.app"
      Facility="local4"
      Severity="info"
      PersistStateInterval="100"
      addMetadata="on")

action(type="omfile"
       File="/var/log/pulse/sensor-proxy/forwarding.log"
       Template="RSYSLOG_TraditionalFileFormat"
       DirCreateMode="0750"
       FileCreateMode="0640")

if (\$programname == 'pulse.audit' or \$programname == 'pulse.app') then {
  action(type="omrelp"
         target="$REMOTE_HOST"
         port="$REMOTE_PORT"
         tls="on"
         tls.caCert="$CA_CERT"
         tls.myCert="$CLIENT_CERT"
         tls.myPrivKey="$CLIENT_KEY"
         queue.type="LinkedList"
         queue.size="50000"
         queue.dequeuebatchsize="500"
         queue.workerthreads="2"
         action.resumeRetryCount="-1")
  stop
}
EOF

systemctl restart rsyslog
echo "Log forwarding enabled to $REMOTE_HOST:$REMOTE_PORT"

# Verification checklist:
# 1. sudo rsyslogd -N1  (syntax check)
# 2. sudo systemctl status rsyslog --no-pager
# 3. tail -f /var/log/pulse/sensor-proxy/forwarding.log
# 4. Confirm new `pulse.audit` events arrive on the remote collector
