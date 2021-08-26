# OplogWatch

Generates a CSV report of oplog statistics for all the clusters in a MongoDB Atlas organization or project.

Each row of the report contains:
* Project ID
* Project Name
* Cluster ID
* Cluster Name
* Oplog Size in MB, if configured, or blank to indicate default size
* Host name of primary node of replica set
* Port of primary node of replica set
* Minimum oplog window over the reporting period, in hours
* Hour of minimum oplog window, a truncated ISO date, e.g. 2021-08-26T10, in GMT
* Average oplog window over the reporting period, in hours
* Maximum oplog generation rate, in GB per hour
* Hour of maximum oplog rate, a truncated ISO date, e.g. 2021-08-26T10, in GMT
* Average oplog generation rate, in GB per hour

Paused clusters are included in the report but will not have any metrics data.

An API Key is required to call the Atlas API. This can be generated at either the organization or project level. 
OplogWatch reads a single key pair from environment variables ATLAS_PUBLIC_KEY and ATLAS_PRIVATE_KEY. 
The report includes all  the clusters in the organization or in a single project, depending on the scope of the API key.

go run oplogwatch.go > path/to/your.csv


## DISCLAIMER
Please note: all tools/ scripts in this repo are released for use "AS IS" without any warranties of any kind, including, 
but not limited to their installation, use, or performance. We disclaim any and all warranties, either express or 
implied, including but not limited to any warranty of noninfringement, merchantability, and/ or fitness for a particular
purpose. We do not warrant that the technology will meet your requirements, that the operation thereof will be 
uninterrupted or error-free, or that any errors will be corrected.

Any use of these scripts and tools is at your own risk. There is no guarantee that they have been through thorough 
testing in a comparable environment and we are not responsible for any damage or data loss incurred with their use.

You are responsible for reviewing and testing any scripts you run thoroughly before use in any non-testing environment.