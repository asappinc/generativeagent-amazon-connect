package config

type AsappConfig struct { // Asapp provided variables
	ApiHost         string `config:"asapp-apiHost,required"`
	ApiId           string `config:"asapp-apiId,required"`
	ApiSecret       string `config:"asapp-apiSecret,required"`
	AssumingRoleArn string `config:"asapp-assumingRoleArn,required"`
}

type ValkeyParameters struct { // Valkey configuration parameters
	CacheNodeType     string `config:"cacheNodeType,required"`
	ReplicaNodesCount int    `config:"replicaNodesCount,required"`
}

type Config struct {
	AccountId          string `config:"accountId,required"`
	Region             string `config:"region,required"`
	ConnectInstanceArn string `config:"connectInstanceArn,required"`
	ObjectPrefix       string `config:"objectPrefix"`
	UseExistingVpcId   string `config:"useExistingVpcId"`

	AttributesToInputVariablesMap  map[string]string `config:"attributesToInputVariablesMap"`
	OutputVariablesToAttributesMap map[string]string `config:"outputVariablesToAttributesMap"`
	SSMLConversions                []SSMLConversion  `config:"ssmlConversions"`

	Asapp                        AsappConfig
	ValkeyParameters             ValkeyParameters
	LambdaProvisionedConcurrency LambdaProvisionedConcurencyConfig `config:"lambdaProvisionedConcurrency"`
}

type SSMLConversion struct {
	SearchFor   string `json:"searchFor"`
	ReplaceWith string `json:"replaceWith"`
}

type LambdaProvisionedConcurencyConfig struct {
	EngageProvisionedConcurrency     int `config:"engageProvisionedConcurrency"`
	PushActionProvisionedConcurrency int `config:"pushActionProvisionedConcurrency"`
	PullActionProvisionedConcurrency int `config:"pullActionProvisionedConcurrency"`
}
