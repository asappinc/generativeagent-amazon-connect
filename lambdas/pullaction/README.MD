# Amazon Connect Lambda GenerativeAgent - PullAction Function

This Lambda function is designed to integrate Amazon Connect with ASAPP GenerativeAgent service. It is invoked by GenerativeAgent Flow Module in Amazon Connect to query the next action to be performed on Amazon Connect side (such as speak text from GenerativeAgent to tranfer call to human agent/other flows)



## Required Environment Variables

The following environment variables must be configured for this Lambda function to work properly:

| Variable           | Description                                           |
| ------------------ | ----------------------------------------------------- |
| `REDIS_URL`        | URL for connection to Redis instance                  |


## Function Flow

1. Receives a contact flow event from Amazon Connect
2. Extracts relevant data from the parameters, specifically guid and companyMarker
3. Poll Redis for next action for this call
4. Perform text replacement for `speak` action as specified in `ssmlConversion.mjs` (if specified) and add `<speak>`/`</speak>` surrounding tags (if any conversions specified)
5. Returns a response with next action (or lack of thereof)

## Packaging code into archive
To package the code and dependencies into single zip archive for uploading to AWS:
 * Run `npm install` to install dependencies into `node_modules` folder
 * Zip `index.mjs`, `types.d.ts` and `node_modules` into single archive

Included `package.sh` script shows examples of the commands that can be run on MacOS 


## Lambda Configuration requirements
See [integration documentation](https://docs.asapp.com/generativeagent/integrate/amazon-connect)