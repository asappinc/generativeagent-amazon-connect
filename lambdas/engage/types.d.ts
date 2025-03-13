export type EngageRequest = {
    namespace: 'amazonconnect'
    guid: string 
    language: 'en-US'
    customerId: string
    amazonConnectParams: AmazonConnectParams

}

export type AmazonConnectParams = {
    streamArn: string
}

export type LambdaResponse = {
    ok: boolean
    asappStatusCode?: number
}