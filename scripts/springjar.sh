#!/usr/bin/env bash

mvn -e -B -U -pl=./ clean dependency:tree deploy

java -Djarmode=layertools -jar $(find *-SNAPSHOT.jar) extract

find dependencies/ -exec touch -t 197001010000 {} \;
find spring-boot-loader/ -exec touch -t 197001010000 {} \;
find snapshot-dependencies/ -exec touch -t 197001010000 {} \;
find application/ -exec touch -t 197001010000 {} \;

cat <<EOF >Dockerfile
FROM repo.dstealer.com:18080/library/jre-8u202-alpine:20221017
COPY dependencies/ /app
COPY spring-boot-loader/ /app
COPY snapshot-dependencies/ /app
COPY application/ /app
WORKDIR /app
CMD ["tini", "--", "java", "org.springframework.boot.loader.JarLauncher"]
EOF

rm -rf dependencies/ spring-boot-loader/ snapshot-dependencies/ application/ Dockerfile
