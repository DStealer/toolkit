#!/usr/bin/env bash
set -ex
mvn -e -B -U -pl=./ clean dependency:tree deploy

rm -rf target/dependencies/ target/spring-boot-loader/ target/snapshot-dependencies/ target/application/ target/Dockerfile target/idfile.txt target/idfile.txt

java -Duser.dir=$(realpath target) -Djarmode=layertools -jar $(realpath $(find target/*-SNAPSHOT.jar)) extract

find target/dependencies/ -exec touch -t 197001010000.00 {} \;
find target/spring-boot-loader/ -exec touch -t 197001010000.00 {} \;
find target/snapshot-dependencies/ -exec touch -t 197001010000.00 {} \;
find target/application/ -exec touch -t 197001010000.00 {} \;

cat <<EOF >target/Dockerfile
FROM repo.dstealer.com:18080/library/jre-8u202-alpine:20221017
COPY dependencies/ /app
COPY spring-boot-loader/ /app
COPY snapshot-dependencies/ /app
COPY application/ /app
WORKDIR /app
CMD ["tini", "--", "java", "org.springframework.boot.loader.JarLauncher"]
EOF

IMAGE_TAG=repo.dstealer.com:18080/library/$(mvn help:evaluate -Dexpression=project.artifactId -q -DforceStdout):$(mvn help:evaluate -Dexpression=project.version -q -DforceStdout)-$(TZ='Asia/Shanghai' date +'%y%m%d%H%M%S')

buildah build --iidfile=target/idfile.txt target

buildah push --rm --digestfile=target/digestfile $(cat target/idfile.txt) docker-daemon:$IMAGE_TAG

buildah rmi $(cat target/idfile.txt)
