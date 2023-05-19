#!/usr/bin/env bash
set -ex
mvn -e -B -U -pl=./ clean dependency:tree deploy

# 低版本springboot不支持
# java -Duser.dir=$(realpath target) -Djarmode=layertools -jar $(realpath $(find target/*-SNAPSHOT.jar)) extract
#清理临时文件
rm -rf target/dependencies/ target/spring-boot-loader/ target/snapshot-dependencies/ target/application/ target/unpacked/ target/Dockerfile target/idfile.txt target/idfile.txt
#拆解jar包
unzip $(realpath $(find target/*-SNAPSHOT.jar)) -d target/unpacked
mkdir -p target/snapshot-dependencies/BOOT-INF/lib target/dependencies/BOOT-INF/ target/spring-boot-loader/ target/application/
find target/unpacked/BOOT-INF/lib/ -name "*-SNAPSHOT.jar" -exec mv {} target/snapshot-dependencies/BOOT-INF/lib/ \;
mv target/unpacked/BOOT-INF/lib/ target/dependencies/BOOT-INF/
mv target/unpacked/org/ target/spring-boot-loader
mv target/unpacked/* target/application/

#排除时间干扰
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

cat $IMAGE_TAG@$(cat target/digestfile) >target/digestfile

buildah rmi $(cat target/idfile.txt)
