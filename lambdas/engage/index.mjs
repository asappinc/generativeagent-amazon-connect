import { default as axios } from 'axios';
import { default as attributesToInputVariables } from './attributesToInputVariables.mjs';

/*
{
    "Details": {
        "ContactData": {
            "Attributes": {
               "exampleAttributeKey1": "exampleAttributeValue1"
              },
            "Channel": "VOICE",
            "ContactId": "4a573372-1f28-4e26-b97b-XXXXXXXXXXX",
            "CustomerEndpoint": {
                "Address": "+1234567890",
                "Type": "TELEPHONE_NUMBER"
            },
            "CustomerId": "someCustomerId",
            "Description": "someDescription",
            "InitialContactId": "4a573372-1f28-4e26-b97b-XXXXXXXXXXX",
            "InitiationMethod": "INBOUND | OUTBOUND | TRANSFER | CALLBACK",
            "InstanceARN": "arn:aws:connect:aws-region:1234567890:instance/c8c0e68d-2200-4265-82c0-XXXXXXXXXX",
            "LanguageCode": "en-US",
            "MediaStreams": {
                "Customer": {
                    "Audio": {
                        "StreamARN": "arn:aws:kinesisvideo::eu-west-2:111111111111:stream/instance-alias-contact-ddddddd-bbbb-dddd-eeee-ffffffffffff/9999999999999",
                        "StartTimestamp": "1571360125131", // Epoch time value
                        "StopTimestamp": "1571360126131",
                        "StartFragmentNumber": "100" // Numberic value for fragment number 
                    }
                }
            },
            "Name": "ContactFlowEvent",
            "PreviousContactId": "4a573372-1f28-4e26-b97b-XXXXXXXXXXX",
            "Queue": {
                   "ARN": "arn:aws:connect:eu-west-2:111111111111:instance/cccccccc-bbbb-dddd-eeee-ffffffffffff/queue/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
                 "Name": "PasswordReset"
                "OutboundCallerId": {
                    "Address": "+12345678903",
                    "Type": "TELEPHONE_NUMBER"
                }
            },
            "References": {
                "key1": {
                    "Type": "url",
                    "Value": "urlvalue"
                }
            },
            "SystemEndpoint": {
                "Address": "+1234567890",
                "Type": "TELEPHONE_NUMBER"
            }
        },
        "Parameters": {}
    },
    "Name": "ContactFlowEvent"
}

*/


/**
 * 
 * @param {import("@types/aws-lambda").ConnectContactFlowEvent} event 
 * @returns {import("./types").LambdaResponse} 
 */
export const handler = async (event) => {
    // console.log(event);

    console.log(`Executing for guid - ${event.Details.ContactData.ContactId}`);

    const inputVariables = {};
    // Map Amazon Connect User Defined Attributes to input variables for use in Engage flows.
    if (event.Details.ContactData.Attributes) {
        for (const [key, value] of Object.entries(event.Details.ContactData.Attributes)) {
            if (attributesToInputVariables[key]) {
                inputVariables[attributesToInputVariables[key]] = value;
            }
        }
    }


    /**
     * @type {import("./types").EngageRequest }
     */
    const req = {
        namespace: 'amazonconnect',
        guid: event.Details.ContactData.ContactId,
        language: 'en-US',
        customerId: event.Details.ContactData.CustomerEndpoint.Address,
        inputVariables,
        amazonConnectParams: {
            streamArn: event.Details.ContactData.MediaStreams.Customer.Audio.StreamARN
        }
    }
    const url = `${process.env['ASAPP_API_HOST']}/mg-genagent/v1/engage`;
    console.log(`ASAPP API request to ${url}: ${JSON.stringify(req)}`);

    /**
     * @type {import("./types").LambdaResponse}
     */
    const response = {
        result: "success",
        asappStatusCode: 0,
        asappErrorResponse: null,
        errorMessage: ""
      };

    let finalStatusCode;
    try {

    const res = await axios({
        method: 'post',
        url,
        headers: {
            "Content-Type": "application/json",
            "asapp-api-id": process.env['ASAPP_API_ID'],
            "asapp-api-secret": process.env['ASAPP_API_SECRET']            
        },
        data: req
    });

        finalStatusCode = res.status;

    } catch (err) {
        if (err.response) {
            console.log(err.response.data);
            finalStatusCode = err.response.status;
            response.errorMessage = `HTTP status ${err.response.status}`;
            if (err.response.data?.error) {
                response.asappErrorResponse = err.response.data.error;
                response.errorMessage = `${err.response.data.error.code} - ${err.response.data.error.message}`;
              }
        }
    }

    response.result = finalStatusCode < 300 ? "success" : "error";
    response.asappStatusCode = finalStatusCode;


    console.log(response);
    return response;

};
