#!/bin/bash

# This script updates test scripts to use unique test node names
# to prevent collision with user nodes

echo "Making test scripts safe by using unique test node names..."

# Add timestamp to test node names to make them unique
TIMESTAMP=$(date +%s)

# Update test-config-validation.sh to use unique names
sed -i "s/\"name\":\"test\"/\"name\":\"test-val-$TIMESTAMP\"/g" /opt/pulse/scripts/test-config-validation.sh
sed -i "s/\"name\":\"duplicate-test\"/\"name\":\"dup-test-$TIMESTAMP\"/g" /opt/pulse/scripts/test-config-validation.sh
sed -i "s/\"name\":\"pbs-test\"/\"name\":\"pbs-test-$TIMESTAMP\"/g" /opt/pulse/scripts/test-config-validation.sh

echo "Test scripts updated with unique node names (suffix: $TIMESTAMP)"
echo ""
echo "You can now safely run tests without affecting production nodes!"