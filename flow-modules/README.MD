# ASAPP GenerativeAgent integration with Amazon Connect - Flow Module


## Integration Documentation
Please see the official [integration documentation](https://docs.asapp.com/generativeagent/integrate/amazon-connect) for complete information about requirements and necessary configurations.


## Instructions

1. Create Lambda functions first as described in the documentation and README for each lambda
2. Write down ARNs for Engage and PullAction Lambdas after they are created
3. Open templates/ASAPPGenerativeAgent.json file on the text editor
4. Replace `LambdaFunctionARN` value from "asapp-generativeagent-pullaction-lambda-arn" to your actual ARN for Lambda function
5. Replace `LambdaFunctionARN` value from "asapp-generativeagent-engage-lambda-arn" to your actual ARN for Lambda function
6. Save as new file file
7. In Amazon Connect console:
   1. Upload prompts from the prompts directory and name the prompt in Amazon Connect the same as the file name (without .wav extension)
       - asappSilence400ms  --> asappSilence400ms.wav
       - asappSilence1s     --> asappSilence1s.wav
       - asappBeepBop       --> asappBeepBop.wav
   2. Import updated json file under Flows --> Modules
   3. Open imported flow module and update prompt references in the following blocks
       -  Wait400msPrompt   --> asappSilence400ms
       -  Wait1sPrompt      --> asappSilence1s
       -  PlayBeepBopShort  --> asappBeepBop



### Pro tip

You can use [jq](https://jqlang.org/download/) utility to update Lambda ARN values as follows
```shell
jq '.Actions |= map(if .Identifier == "PullAction" then .Parameters.LambdaFunctionARN = "pull-action-arn" elif .Identifier == "Engage" then .Parameters.LambdaFunctionARN = "engage-arn" else . end)' template/ASAPPGenerativeAgent.json > my_ASAPPGenerativeAgent.json 
```

Where:
 - pull-action-arn - is the actual ARN for Lambda Pull Action function
 - engage-arn      - is the actual ARN for Lambda Engage function