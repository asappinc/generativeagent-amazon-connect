# Amazon Connect Lambda GenerativeAgent - PushAction Function

This Lambda function is designed to integrate Amazon Connect with ASAPP GenerativeAgent service. It is invoked by ASAPP services to communicate the state of GenerativeAgent (such as whether processing started or finished) and next action to be performed on Amazon Connect side (such as speak text from GenerativeAgent to tranfer call to human agent/other flows)



## Required Environment Variables

The following environment variables must be configured for this Lambda function to work properly:

| Variable           | Description                                           |
| ------------------ | ----------------------------------------------------- |
| `REDIS_URL`        | URL for connection to Redis instance                  |


## Function Flow

1. Function is invoked by ASAPP with JSON event payload
2. Put event payload as string to Redis
3. Returns a response indicating success or failure


## Packaging code into archive
To package the code and dependencies into single zip archive for uploading to AWS:
 * Run `npm install` to install dependencies into `node_modules` folder
 * Zip `index.mjs`, `types.d.ts` and `node_modules` into single archive

Included `package.sh` script shows examples of the commands that can be run on MacOS 


## Lambda Configuration requirements
See [integration documentation](https://docs.asapp.com/generativeagent/integrate/amazon-connect)