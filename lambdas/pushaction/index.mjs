import { GlideClient, Transaction } from "@valkey/valkey-glide";

const valkeyTTLSeconds = 21600;

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
        const valkeyHost = process.env['VALKEY_HOST'];
        if (!valkeyHost) {
            throw new Error("VALKEY_HOST environment variable must be set");
        }

        const valkeyPort = process.env['VALKEY_PORT'];
        if (!valkeyPort) {
            throw new Error("VALKEY_PORT environment variable must be set");
        }

        const host = valkeyHost;
        const port = parseInt(valkeyPort, 10) || 6379;

        client = await GlideClient.createClient({
            addresses: [
                {
                    host: host,
                    port: port,
                },
            ],
            clientName: "pushaction_client",
        });
    } catch (err) {
        return {
            ok: false,
            errorMessage: `error connecting to Valkey - ${err}`
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
                const transaction = new Transaction();
                transaction.rpush(key, JSON.stringify(event));
                transaction.expire(key, valkeyTTLSeconds);
                await client.exec(transaction);
                break;

            default:
                console.log(`Received event for unsupported action - ${JSON.stringify(event)}`);
        }


    } catch (err) {
        console.error(err);
        return {
            ok: false,
            errorMessage: `error processing action ${event.action} - ${err}`
        };
    } finally {
        if (client && !client.isClosed) {
            client.close();
        }
    }

    return { ok: true };
};
