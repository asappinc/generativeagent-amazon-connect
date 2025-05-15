# Amazon Connect Lambda GenerativeAgent - PullAction Function

This Lambda function is designed to integrate Amazon Connect with ASAPP GenerativeAgent service. It is invoked by GenerativeAgent Flow Module in Amazon Connect to query the next action to be performed on Amazon Connect side (such as speak text from GenerativeAgent to tranfer call to human agent/other flows)



## Required Environment Variables

The following environment variables must be configured for this Lambda function to work properly:

| Variable           | Description                                           |
| ------------------ | ----------------------------------------------------- |
| `VALKEY_HOST`      | Hostname for the Valkey instance                      |
| `VALKEY_PORT`      | Port number for the Valkey instance                   |

## Function Flow

1. Receives a contact flow event from Amazon Connect
2. Extracts relevant data from the parameters, specifically guid and companyMarker
3. Poll Valkey for next action for this call
4. Perform text replacement for `speak` action as specified in `ssmlConversion.mjs` (if specified) and add `<speak>`/`</speak>` surrounding tags (if any conversions specified)
5. Returns a response with next action (or lack of thereof)

## Packaging code into archive
To package the code and dependencies into a single zip archive for uploading to AWS:
 * Use Docker to install dependencies compatible with Amazon Linux and Node.js 22:
   ```bash
   docker run --rm --platform linux/amd64 -v "$PWD":/var/task -w /var/task amazonlinux:2023 /bin/bash -c "
       curl -sL https://rpm.nodesource.com/setup_22.x | bash - && \
       yum install -y nodejs && \
       npm install
   "
 * Zip `node_modules`, `index.mjs`, `types.d.ts` and `ssmlConversions.mjs` into a single archive

Included `package.sh` script shows examples of the commands that can be run on macOS


## Lambda Configuration requirements
See [integration documentation](https://docs.asapp.com/generativeagent/integrate/amazon-connect)