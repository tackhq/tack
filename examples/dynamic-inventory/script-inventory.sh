#!/bin/sh
# Example: Script-based dynamic inventory
# Make executable and pass as: tack run playbook.yaml -i ./script-inventory.sh
#
# The script receives --list as its argument and should output JSON or YAML
# in Tack's inventory format.

cat <<'EOF'
{
  "hosts": {
    "web1": {
      "vars": {
        "region": "us-east-1",
        "role": "web"
      }
    },
    "web2": {
      "vars": {
        "region": "us-west-2",
        "role": "web"
      }
    },
    "db1": {
      "vars": {
        "region": "us-east-1",
        "role": "database"
      }
    }
  },
  "groups": {
    "webservers": {
      "hosts": ["web1", "web2"],
      "connection": "ssh",
      "vars": {
        "app_port": 8080
      }
    },
    "databases": {
      "hosts": ["db1"],
      "connection": "ssh"
    }
  }
}
EOF
