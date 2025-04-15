// This file contains the SSML conversion rules that apply to text to be spoken as a result of a "speak" action.
// No validation is done on replacements, the text is replaced using string.replaceAll() with all instances of the searchFor string being replaced with the replaceWith string.
// Sample replacements:
// {
//     searchFor: 'ASAPP',
//     replaceWith:'<phoneme alphabet="ipa" ph="eɪˈsæp">ASAPP</phoneme>',
// }
// The surrounding <speak></speak> tags are added automatically by the lambda function if any replacements specified. Note that Amazon Connect Flow block must be set to "SSML" for the text to be spoken as SSML (this is done by CDK code automatically)
// Note that not all voices support all SSML tags, check https://docs.aws.amazon.com/polly/latest/dg/supportedtags.html for details.
// SSML tags for English US are described at https://docs.aws.amazon.com/polly/latest/dg/ph-table-english-us.html
// By default list of replacements is empty and no replacements are done and text is spoken as is.
export default
[

]