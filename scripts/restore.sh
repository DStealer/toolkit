#!/usr/bin/env bash

host=127.0.0.1
port=3306
user=root
password=root123

set -e

cd "$(dirname "$0")" || exit 1

# shellcheck disable=SC2010
for filename in $(ls ./ | grep -Ei "sql.gz"); do
  echo "handle ${filename}"
  gunzip <"${filename}" | mysql -h ${host} -P ${port} -u ${user} --max-allowed-packet="$(512×1024)" --connect-timeout=3600 -p"${password}" -e
done
