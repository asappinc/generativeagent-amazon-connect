import * as redis from 'redis'
import {default as ssmlConversions} from './ssmlConversions.mjs';
const redisTTLSeconds = 21600;
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
        "Parameters": {"companyMarker": "companyMarker111",
               "guid": "4a573372-1f28-4e26-b97b-XXXXXXXXXXX"
        }
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

    /**
     * @type  {import("./types").LambdaResponse} 
     */
    const response = {
        next: 'none',
        playBeepBop: 0
    }

    for (const param of ['companyMarker', 'guid']) {
        if (!event.Details.Parameters[param]) {

            response.next = 'error'
            response.text = `missing ${param} parameter`
            return response;
        }

    }

    console.log(`Executing for guid - ${event.Details.Parameters.guid}`);
    let client;
    try {
        client = await redis.createClient({
            url: process.env['REDIS_URL']
        })
            .on('error', err => console.log('Redis Client Error', err))
            .connect();

    } catch (err) {
        response.next = 'error'
        response.text = `error connecting to redis - ${err}`
        return response;

    }


    const key = `asappActions:${event.Details.Parameters.companyMarker}:${event.Details.Parameters.guid}`;
    const keyBeepBopCounter = `asappStates:beepBopCounter:${event.Details.Parameters.companyMarker}:${event.Details.Parameters.guid}`;

    try {
        
        const [beepBopCounter, expireResponse] = await client.multi().incr(keyBeepBopCounter).expire(keyBeepBopCounter, redisTTLSeconds).exec();
        
        console.log(`beepBopCounter for ${event.Details.Parameters.guid}} is ${beepBopCounter}; expire response is ${expireResponse}`);
        response.playBeepBop = beepBopCounter % 6 == 1 ? 1 : 0;
        console.log(`set playBeepBop for guid ${event.Details.Parameters.guid}} to ${response.playBeepBop}`);
        

        console.log(`retrieving from next action from key ${key}`);
        let redisResponse = await client.lPop(key);
        console.log(redisResponse);
        if (!redisResponse) {
            return response;
        }

        /**
         * @type {import("./types").BaseAction}
         */
        const nextAction = JSON.parse(redisResponse);

        switch (nextAction.action) {
            case 'speak':
                response.next = nextAction.action;
                response.text = ssmlConvert(nextAction.speakParams.text);
                return response
            case 'transferToAgent':
            case 'transferToSystem':
            case 'disengage':
                response.next = nextAction.action;
                if (nextAction.transferToAgentParams && nextAction.transferToAgentParams.outputVariables) response.outputVariables = nextAction.transferToAgentParams.outputVariables;
                if (nextAction.transferToSystemParams && nextAction.transferToSystemParams.outputVariables) response.outputVariables = nextAction.transferToSystemParams.outputVariables;
                return response
            case 'processingStart':
            case 'processingEnd':
                response.next = nextAction.action;
                return response
            default:
                console.error(`unexpected action ${nextAction.action} from redis`);
                break;
        }




    } catch (err) {
        console.error(err);
        response.next = 'error';
        response.text = `error retrieving action for ${key} - ${err}`;
    }

    return response;

};


function ssmlConvert(text) {
    let ssmlText = text;
    let convertToSSML = false;
    let ret = text;
    if (ssmlConversions && ssmlConversions.length > 0) {
        convertToSSML = true;

        for (const conversion of ssmlConversions) {
            const searchFor = RegExp(conversion.searchFor, 'gi');
            if (ssmlText.match(searchFor)) {
                ssmlText = ssmlText.replaceAll(searchFor, conversion.replaceWith);
            }
        }
    }

    if (convertToSSML) {
        ret = `<speak>${ssmlText}</speak>`;
    }

    return ret;
}