#!/usr/bin/env bash
set -e
# shellcheck disable=SC2046
cd $(dirname "$0") || exit 1

###+++自定义参数开始
#应用文件名称,必须
PROJECT_NAME="{{.ProjectFileName}}"
#使用的jdk,必须
JAVA_BIN="/usr/bin/env java"
#jvm参数,可选
JAVA_OPTS="-Xms64M -Xmx256M -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=./dump"
#java agent,可选
#AGENT_OPTS="-javaagent:/fakepath/skywalking-agent.jar -Dskywalking.agent.service_name=xxx -Dskywalking.collector.servers=ip:port"
#spring profile,可选
SPRING_PROFILE="--spring.profiles.active=default"
#spring config 目录配置,可选
#spring boot 1.x.x
#spring 2.x.x 会自动搜索 classpath:/ classpath:/config/ file:./ file:./config/ file:./config/*/; 如果启用需要配合 spring.config.use-legacy-processing
SPRING_CONFIG="--spring.config.location=file:./config/"
#其他参数,可选
OTHER_OPTS=""
###---自定义参数结束

#工作目录
TARGET_DIR=$(pwd)
#启动命令和参数
COMMAND_ARGS="${JAVA_BIN} ${JAVA_OPTS:-} ${AGENT_OPTS:-} -jar ${TARGET_DIR}/${PROJECT_NAME} ${SPRING_PROFILE:-} ${SPRING_CONFIG:-} ${OTHER_OPTS:-}"
#控制台输出文件
OUT_FILE="${TARGET_DIR}/${PROJECT_NAME:0:${#PROJECT_NAME}-4}.out"

case $1 in
start)
  if [ -f "${OUT_FILE}" ]; then
    echo "Remove ${OUT_FILE} ..."
    rm -f "${OUT_FILE}"
  fi
  echo "Starting ${PROJECT_NAME} ... "
  # shellcheck disable=SC2009
  pid=$(ps ax | grep ${PROJECT_NAME} | grep "${TARGET_DIR}" | grep java | grep -v grep | awk '{print $1}')
  if [ -n "${pid}" ]; then
    echo "${PROJECT_NAME}(${pid}) running already."
    exit 1
  fi
  # shellcheck disable=SC2086
  nohup ${COMMAND_ARGS} >/dev/null 2>&1 &

  echo "${PROJECT_NAME} is starting..."
  sleep 5
  # shellcheck disable=SC2009
  pid=$(ps ax | grep ${PROJECT_NAME} | grep "${TARGET_DIR}" | grep java | grep -v grep | awk '{print $1}')
  if [ -z "${pid}" ]; then
    echo "${PROJECT_NAME} start failed,please check your logs"
    exit 1
  fi
  echo "${PROJECT_NAME}(${pid}) start success"

  # shellcheck disable=SC2046
  echo -e "last start at:$(date)\n$(md5sum ${PROJECT_NAME})" >"${TARGET_DIR}/last_startup.txt"
  ;;
start-foreground)
  echo "Starting ${PROJECT_NAME} ... "
  # shellcheck disable=SC2009
  pid=$(ps ax | grep ${PROJECT_NAME} | grep "${TARGET_DIR}" | grep java | grep -v grep | awk '{print $1}')
  if [ -n "${pid}" ]; then
    echo "${PROJECT_NAME} running already."
    exit 1
  fi
  ${COMMAND_ARGS} >"${OUT_FILE}" 2>&1 &
  echo "${PROJECT_NAME} is starting"
  sleep 5
  # shellcheck disable=SC2009
  pid=$(ps ax | grep ${PROJECT_NAME} | grep "${TARGET_DIR}" | grep java | grep -v grep | awk '{print $1}')
  if [ -z "${pid}" ]; then
    echo "${PROJECT_NAME} start failed,please check ${OUT_FILE}"
    less "${OUT_FILE}"
    exit 1
  fi
  echo "${PROJECT_NAME}(${pid}) start success"
  # shellcheck disable=SC2046
  echo -e "last start at:$(date)\n$(md5sum ${PROJECT_NAME})" >"${TARGET_DIR}/last_startup.txt"
  tail -f -n 100 --pid "${pid}" "${OUT_FILE}"
  ;;
stop)
  # shellcheck disable=SC2009
  pid=$(ps ax | grep ${PROJECT_NAME} | grep "${TARGET_DIR}" | grep java | grep -v grep | awk '{print $1}')
  if [ -z "${pid}" ]; then
    echo "No ${PROJECT_NAME} running."
    exit 0
  fi
  echo "The ${PROJECT_NAME}(${pid}) is running..."
  # shellcheck disable=SC2086
  kill -9 ${pid}
  echo "Send shutdown request to ${PROJECT_NAME}(${pid}) OK"
  sleep 5
  # shellcheck disable=SC2009
  pid=$(ps ax | grep ${PROJECT_NAME} | grep "${TARGET_DIR}" | grep java | grep -v grep | awk '{print $1}')
  if [ -n "${pid}" ]; then
    echo "${PROJECT_NAME} stop failed"
    exec -1
  fi
  echo "${PROJECT_NAME} stop success"
  ;;
restart)
  shift
  # shellcheck disable=SC2068
  "$0" stop ${@}
  sleep 1
  # shellcheck disable=SC2068
  "$0" start ${@}
  ;;
status)
  # shellcheck disable=SC2009
  pid=$(ps ax | grep ${PROJECT_NAME} | grep "${TARGET_DIR}" | grep java | grep -v grep | awk '{print $1}')
  if [ -z "${pid}" ]; then
    echo "No ${PROJECT_NAME} running."
    exit 0
  fi
  if [ -f "${TARGET_DIR}/last_startup.txt" ]; then
    cat "${TARGET_DIR}/last_startup.txt"
  fi
  ps --no-heading -Fp "${pid}"
  netstat -nap | grep "${pid}"/java | grep LISTEN
  ;;
backup)
  if [ ! -d "${TARGET_DIR}/backup" ]; then
    mkdir "${TARGET_DIR}/backup"
  fi

  find "${TARGET_DIR}/backup" -name "${PROJECT_NAME}-all-*" | sort -r | sed -n '7,$p' | xargs -I {} rm -f {}

  TMP_FILE=$(mktemp)

  tar -cf "${TMP_FILE}" "${PROJECT_NAME}" "${SPRING_CONFIG#*file:}"

  TARGET_NAME="${PROJECT_NAME}-all-$(date +'%y%m%d%H%M%S')-$(md5sum "${TMP_FILE}" | awk '{print $1}').tar"

  mv "${TMP_FILE}" "${TARGET_DIR}/backup/${TARGET_NAME}"

  echo "backup ${TARGET_DIR}/backup/${TARGET_NAME} complete"
  ;;
backup-conf)
  if [ ! -d "${TARGET_DIR}/backup" ]; then
    mkdir "${TARGET_DIR}/backup"
  fi

  find "${TARGET_DIR}/backup" -name "${PROJECT_NAME}-conf-*" | sort -r | sed -n '7,$p' | xargs -I {} rm -f {}

  TMP_FILE=$(mktemp)

  tar -cf "${TMP_FILE}" "${SPRING_CONFIG#*file:}"

  TARGET_NAME="${PROJECT_NAME}-conf-$(date +'%y%m%d%H%M%S')-$(md5sum "${TMP_FILE}" | awk '{print $1}').tar"

  mv "${TMP_FILE}" "${TARGET_DIR}/backup/${TARGET_NAME}"

  echo "backup ${TARGET_DIR}/backup/${TARGET_NAME} complete"
  ;;
backup-jar)
  if [ ! -d "${TARGET_DIR}/backup" ]; then
    mkdir "${TARGET_DIR}/backup"
  fi

  find "${TARGET_DIR}/backup" -name "${PROJECT_NAME}-jar-*" | sort -r | sed -n '7,$p' | xargs -I {} rm -f {}

  TMP_FILE=$(mktemp)

  tar -cf "${TMP_FILE}" "${PROJECT_NAME}"

  TARGET_NAME="${PROJECT_NAME}-jar-$(date +'%y%m%d%H%M%S')-$(md5sum "${TMP_FILE}" | awk '{print $1}').tar"

  mv "${TMP_FILE}" "${TARGET_DIR}/backup/${TARGET_NAME}"

  echo "backup ${TARGET_DIR}/backup/${TARGET_NAME} complete"
  ;;
*)
  echo "Usage: $0 {start|start-foreground|stop|restart|status|backup|backup-jar|backup-conf}" >&2
  ;;
esac
