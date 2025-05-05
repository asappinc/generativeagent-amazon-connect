export type EngageRequest = {
    namespace: 'amazonconnect'
    guid: string 
    language: 'en-US'
    customerId: string
    amazonConnectParams: AmazonConnectParams
    inputVariables: Record<string, string>

}

export type AmazonConnectParams = {
    streamArn: string
}

export type LambdaResponse = {
    result: "success" | "error"
    asappStatusCode?: number,
    asappErrorResponse: AsappErrorResponse
    errorMessage?: string
}


export type AsappErrorResponse = {
    requestId: string
    message: string
    code: string
}