export type BaseAction = {
    action: 'speak' | 'transferToAgent' | 'transferToSystem' | 'processingStart' | 'processingEnd' | 'disengage'
    companyMarker: string
    guid: string
    speakParams?: SpeakParams
    transferToAgentParams?: TransferToAgentParams
    transferToSystemParams?: TransferToSystemParams

}


export type SpeakParams = {
    text: string
}

export type TransferToAgentParams = {
    outputVariables?: object
}

export type TransferToSystemParams = {
    outputVariables?: object
}

export type LambdaResponse = {
    next: 'none' | 'speak' | 'transferToAgent' | 'transferToSystem' | 'processingStart' | 'processingEnd' | 'disengage' | 'error'
    text?: string
    outputVariables?: { [key: string]: string },
    playBeepBop: 0 | 1
}