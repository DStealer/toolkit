#!/usr/bin/env bash

host=127.0.0.1
port=3306
user=root
password=root123

set -e

cd "$(dirname "$0")" || exit 1

for dbname in $(mysql -h ${host} -P ${port} -u ${user} -p"${password}" -N -e "show databases;" |
  grep -Evi "information_schema|performance_schema|mysql|sys|test"); do
  echo "handle ${dbname}"
  mysqldump -h ${host} -P ${port} -u ${user} -p"${password}" \
    --flush-privileges \
    --flush-logs \
    --hex-blob \
    --triggers \
    --routines \
    --events \
    --routines \
    --single-transaction \
    --master-data=2 -B "${dbname}" | gzip >"${dbname}".sql.gz
done
