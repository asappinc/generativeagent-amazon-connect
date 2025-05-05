# Change log

All notable changes to this project will be documented in this file.
 
The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [1.2.0] - 2024-05-05
### Added
 - Flow Module: Handle failures during Engage API call
 - Lambdas: Refactor outputs from Engage Lambda so Connect can act on failures
 - CDK: Provision Lambdas with version, alias and optional provisioned concurrency settings
 - CDK: Explicitly create IAM role/policy used for custom resources (like Connect prompts) to avoid intermittent race conditions

## [1.1.1] - 2024-04-16
### Changed
 - Readme update with SSML text replacement documentation

## [1.1.0] - 2024-04-15
### Added
 - Add support to specify SSML text replacements for text returned by GenerativeAgent

### Changed
 - CDK: Prefix prompt names with overall prefix suffix 

## [1.0.2] - 2024-03-26
### Changed
 - CDK: Set Engage lambda timeout to 15 seconds to account for delays in cross region scenarios


## [1.0.1] - 2024-03-26
### Added
 - CDK: Add support to use existing VPC instead of always creating new one
 - CDK: Add support for customer managed KMS key used for Kinesis streaming (permission added to IAM role)

### Changed
 - Module: Add default configuration to ExtractOutputVariables block to allow empty outputVariables in CDK


## [1.0.0] - 2024-03-24
### Added
 - GenerativeAgent Input and Output variables support
 - Refactor CDK code structure to be inline with Golang conventions
 - Provide sample configuration file


## [0.1.0] - 2024-03-18
### Added
 - Initial release 