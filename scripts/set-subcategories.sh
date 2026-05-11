#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# Sets subcategory frontmatter in generated docs.
# Run after tfplugindocs generate.

set -euo pipefail

DOCS_DIR="$(cd "$(dirname "$0")/.." && pwd)/docs"

set_subcategory() {
  local file="$1" subcategory="$2" tmpfile
  tmpfile=$(mktemp)
  sed "s/subcategory: \"\"/subcategory: \"${subcategory}\"/" "$file" > "$tmpfile"
  mv "$tmpfile" "$file"
}

# Format: type|name|subcategory
while IFS='|' read -r type name subcategory; do
  [ -z "$type" ] && continue
  file="$DOCS_DIR/$type/$name.md"
  if [ -f "$file" ]; then
    set_subcategory "$file" "$subcategory"
  else
    echo "Warning: $file not found" >&2
  fi
done <<'EOF'
resources|standard_network|Networks
resources|standard_gateway|Gateways
resources|standard_wireguard_tunnel|Tunnels
data-sources|standard_gateway|Gateways
data-sources|regions|Regions
EOF
