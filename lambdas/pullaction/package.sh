#!/bin/zsh
# Use Docker to install dependencies for Amazon Linux x64 and Node.js 22
docker run --rm --platform linux/amd64 -v "$PWD":/var/task -w /var/task amazonlinux:2023 /bin/bash -c "
    curl -sL https://rpm.nodesource.com/setup_22.x | bash - && \
    yum install -y nodejs && \
    npm install
"

if [ ! -d "node_modules" ]; then
    echo 'Error: node_modules folder not found on host. Check Docker volume mounting.'
    exit 1
fi

zip -X -r lambda.zip node_modules index.mjs types.d.ts ssmlConversions.mjs