package quickstart

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/iancoleman/orderedmap"
)

func UpdateResourcesARN(data *orderedmap.OrderedMap, region, accountId, connectInstanceArn string, promptArnMap, lambdaFunctionsArnMap map[string]string) {
	for _, key := range data.Keys() {
		value, _ := data.Get(key)
		strValue, ok := value.(string)
		if ok {
			newPromptValue, newPromptKeyExists := promptArnMap[strValue]
			if key == "Identifier" && newPromptKeyExists { // Updates the ARN of the Prompts whose Identifier is present in the map
				parametersValue, _ := data.Get("Parameters")
				parametersValueMap := parametersValue.(orderedmap.OrderedMap)
				parametersValueMap.Set("PromptId", newPromptValue)
				continue
			}
			newLambdaFunctionValue, newLambdaKeyExists := lambdaFunctionsArnMap[strValue]
			if key == "Identifier" && newLambdaKeyExists { // Updates the ARN of the Lambda functions whose Identifier is present in the map
				parametersValue, _ := data.Get("Parameters")
				parametersValueMap := parametersValue.(orderedmap.OrderedMap)
				parametersValueMap.Set("LambdaFunctionARN", newLambdaFunctionValue)
				continue
			}
			if arn.IsARN(strValue) {
				arnValue, err := arn.Parse(strValue)
				if err != nil {
					panic(err)
				}
				arnValue.Region = region
				arnValue.AccountID = accountId
				data.Set(key, arnValue.String())
			}
			continue
		}
		if nestedMap, ok := value.(orderedmap.OrderedMap); ok {
			// Recursively call for nested ordered maps
			UpdateResourcesARN(&nestedMap, region, accountId, connectInstanceArn, promptArnMap, lambdaFunctionsArnMap)
		} else if nestedSlice, ok := value.([]interface{}); ok {
			for _, item := range nestedSlice {
				if itemMap, ok := item.(orderedmap.OrderedMap); ok {
					// Recursively call for each map in the slice
					UpdateResourcesARN(&itemMap, region, accountId, connectInstanceArn, promptArnMap, lambdaFunctionsArnMap)
				}
			}
		}
	}
}

//	{
//		"Parameters": {
//		  "Attributes": {},
//		  "TargetContact": "Current"
//		},
//		"Identifier": "ExtractOutputVariables",
//		"Type": "UpdateContactAttributes",
//		"Transitions": {
//		  "NextAction": "DidModuleStartMediaStreaming2",
//		  "Errors": [
//			{
//			  "NextAction": "DidModuleStartMediaStreaming2",
//			  "ErrorType": "NoMatchingError"
//			}
//		  ]
//		}
//	  }
func UpdateExtractOutputVariables(data *orderedmap.OrderedMap, outputVariablesToAttributesMap map[string]string) {

	actions, ok := data.Get("Actions")
	if !ok {
		return
	}

	for _, val := range actions.([]any) {
		action := val.(orderedmap.OrderedMap)
		identifier, ok := action.Get("Identifier")
		if !ok || identifier.(string) != "ExtractOutputVariables" {
			continue
		}

		parameters, ok := action.Get("Parameters")
		if !ok {
			break
		}
		parametersMap := parameters.(orderedmap.OrderedMap)
		attributes, ok := parametersMap.Get("Attributes")
		if !ok {
			break
		}
		attributesMap := attributes.(orderedmap.OrderedMap)
		for outputVariable, targetAttribute := range outputVariablesToAttributesMap {
			attributesMap.Set(targetAttribute, fmt.Sprintf("$.External.outputVariables.%s", outputVariable))
		}
		parametersMap.Set("Attributes", attributesMap)
		action.Set("Parameters", parametersMap)
		break

	}

}
