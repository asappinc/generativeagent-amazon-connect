export type BaseAction = {
    action: 'speak' | 'transferToAgent' | 'transferToSystem' | 'processingStart' | 'processingEnd' | 'disengage'
    companyMarker: string
    guid: string
    speakParams?: SpeakParams
    transferToAgentParams?: TransferToAgentParams
    transferToSystemParams?: TransferToSystemParams
    reason?: string

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