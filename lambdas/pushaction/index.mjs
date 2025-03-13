import * as redis from 'redis'

const redisTTLSeconds = 21600;

/*
{
    "action": "speak",
    "companyMarker": "sadevelop",
    "guid": "123445",
    "speakParams": {
        "text": "this is my text"
    }
}

*/

/**
 * 
 * @param {import("./types").BaseAction} event 
 * @returns {object}
 */
export const handler = async (event) => {

    for (const field of ['action', 'companyMarker', 'guid']) {
        if (!event[field]) {
            return {
                ok: false,
                errorMessage: `missing ${field} field`
            }
        }
            
    }  
    

    let client;
    try {
        client = await redis.createClient({
            url: process.env['REDIS_URL']
        })
            .on('error', err => console.log('Redis Client Error', err))
            .connect();

    } catch (err) {
        return {
            ok: false,
            errorMessage: `error connecting to redis - ${err}`
        }

    }


    const key = `asappActions:${event.companyMarker}:${event.guid}`
    try {
        switch (event.action) {
            case 'speak':
            case 'transferToAgent':
            case 'transferToSystem':
            case 'processingStart':
            case 'processingEnd':
            case 'disengage':
                await client.multi().rPush(key, JSON.stringify(event)).expire(key, redisTTLSeconds).exec();
                await client.quit()
                break;

            default:
                console.log(`Received event for unsupported action - ${JSON.stringify(event)}`);
        }


    } catch (err) {
        console.error(err);
        return {
            ok: false,
            errorMessage: `error processing action ${event.action} - ${err}`
        }


    }

    return { ok: true };

};
