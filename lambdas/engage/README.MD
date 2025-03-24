# Amazon Connect Lambda GenerativeAgent - Engage Function

This Lambda function is designed to integrate Amazon Connect with ASAPP GenerativeAgent service. It sends REST API request to ASAPP to engage ASAPP GenerativeAgent for an Amazon Connect call. The necessary call information is extracted from the Contact Flow event. 

This Lambda function must be called from Amazon Connect flow. 


## Required Environment Variables

The following environment variables must be configured for this Lambda function to work properly:

| Variable           | Description                                                |
| ------------------ | ---------------------------------------------------------- |
| `ASAPP_API_HOST`   | Base URL for the ASAPP API (e.g., `https://api.asapp.com`) |
| `ASAPP_API_ID`     | API ID for authentication with ASAPP services              |
| `ASAPP_API_SECRET` | API Secret key for authentication with ASAPP services      |

## Function Flow

1. Receives a contact flow event from Amazon Connect
2. Extracts relevant data including the contact ID, customer phone number, and media stream ARN
3. Extracts Contact Attributes and maps them to inputVariables for GenerativeAgent using mapping in `attributesToInputVariables.mjs`
4. Makes a POST request to the ASAPP API
5. Returns a response indicating success or failure

## Packaging code into archive
To package the code and dependencies into single zip archive for uploading to AWS:
 * Run `npm install` to install dependencies into `node_modules` folder
 * Zip `index.mjs`, `attributesToInputVariables.mjs`, `types.d.ts` and `node_modules` into single archive

Included `package.sh` script shows examples of the commands that can be run on MacOS 


## Lambda Configuration requirements
See [integration documentation](https://docs.asapp.com/generativeagent/integrate/amazon-connect)


### Sample attributesToInputVariables.mjs contents

```typescript
// Maps Amazon Connect User Defined Attributes to input variables for use in Engage flows. Key is the Connect attribute name, value is the GenerativeAgent input variable name.
export default 
{
    "AccountNumber":"CustomerAccountNumber"
}
```

In this case Amazon Connect Contact has User Defined attribute called `AccountNumber`. The value of that attribute (if it exists) will be sent as `CustomerAccountNumber` input variable to GenerativeAgent by this lambda. E.g. GenerativeAgent will see and can reference this input variable value as `CustomerAccountNumber` .

See [Input Variables documentation](https://docs.asapp.com/generativeagent/configuring/tasks-and-functions/input-variables) of GenerativeAgent to learn how to reference input variables in GenerativeAgent tasks