package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mongodb-forks/digest"
	"go.mongodb.org/atlas/mongodbatlas"
)

func main() {

	d := false

	t := digest.NewTransport(os.Getenv("ATLAS_PUBLIC_KEY"), os.Getenv("ATLAS_PRIVATE_KEY"))
	tc, err := t.Client()
	if err != nil {
		log.Fatalf(err.Error())
	}

	client := mongodbatlas.NewClient(tc)

	fmt.Println("Project ID,Project Name,Cluster ID, Cluster Name,Oplog Size (MB),Primary Host,Port,Min Window (hrs), Min Hour (Z), Avg Window (hrs), Max Rate (GB / hr), Max Hour (Z), Avg Rate (GB / hr)")
	moreProjects := true
	projPage := 1
	itemsPerPage := 50

	// maps as sets to store the unique timestamp values. e.g. if hourly, keys will be, e.g., 2021-08-19T18
	oplogWindowTimes := make(map[string]bool)
	oplogRateTimes := make(map[string]bool)

	for moreProjects {
		var projListOptions = mongodbatlas.ListOptions{
			PageNum:      projPage,
			ItemsPerPage: itemsPerPage,
			IncludeCount: true,
		}

		// Get projects
		projects, _, err := client.Projects.GetAllProjects(context.Background(), &projListOptions)
		if err != nil {
			log.Fatalf("Projects.GetAllProjects returned error: %v", err)
		}

		if d {
			fmt.Println("Projects TotalCount: ", projects.TotalCount)
			fmt.Println("projPage: ", projPage)
		}

		if projPage*itemsPerPage >= projects.TotalCount {
			moreProjects = false
		}
		projPage++

		// Iterate over projects
		for _, project := range (*projects).Results {
			if d {
				fmt.Println("")
				fmt.Println("**********************************************")
				fmt.Println("")
				fmt.Println("Project ID: ", project.ID)
				fmt.Println("Project Name: ", project.Name)
				fmt.Println("")
			}

			projectFields := []string{project.ID, project.Name}
			//fmt.Println(strings.Join(projectFields, ","))

			// Get clusters in the project
			clusters, _, err := client.Clusters.List(context.Background(), project.ID, nil)
			if err != nil {
				log.Fatalf("Clusters.List returned error: %v", err)
			}

			clusterMap := make(map[string]*mongodbatlas.Cluster)
			clusterOplogSizeMap := make(map[string]int64)

			// Iterate over clusters
			for _, cluster := range clusters {

				if d {
					fmt.Println("Cluster Name: ", cluster.Name)
					fmt.Println("Cluster ID: ", cluster.ID)
					fmt.Println("Cluster ClusterType: ", cluster.ClusterType)
				}

				clusterMap[strings.ToLower(cluster.Name)] = &cluster

				processArgs, _, err := client.Clusters.GetProcessArgs(context.Background(), project.ID, cluster.Name)
				if err != nil {
					log.Fatalf("Clusters.GetProcessArgs returned error: %v", err)
				}

				if processArgs.OplogSizeMB != nil {
					clusterOplogSizeMap[strings.ToLower(cluster.Name)] = *processArgs.OplogSizeMB
				}
			}

			// Get processes in the project
			processes, _, err := client.Processes.List(context.Background(), project.ID, nil)
			if err != nil {
				log.Fatalf("Processes.List returned error: %v", err)
			}

			// Iterate over processes
			for _, process := range processes {

				// For now, we are only watching primaries
				if process.TypeName == "REPLICA_PRIMARY" {
					if d {
						fmt.Println("Host: ", process.Hostname)
						fmt.Println("Port: ", process.Port)
						fmt.Println("ID: ", process.ID)
						fmt.Println("ReplicaSetName: ", process.ReplicaSetName)
						fmt.Println("ShardName: ", process.ShardName)
						fmt.Println("UserAlias: ", process.UserAlias)
					}

					// not sure yet why this can be empty. Maybe during cluster creation ?
					if len(process.UserAlias) == 0 {
						continue
					}

					// extract lower-case cluster name from Process.UserAlias and look up in clusterMap
					lastIndex := strings.LastIndex(process.UserAlias, "-shard-")
					lcClusterName := process.UserAlias[0:lastIndex]
					thisCluster := clusterMap[lcClusterName]
					//fmt.Println("Cluster name: ", thisCluster.Name)

					oplogSize := clusterOplogSizeMap[lcClusterName]
					oplogSizeStr := ""
					if oplogSize > 0 {
						oplogSizeStr = strconv.Itoa(int(oplogSize))
					}

					processFields := []string{thisCluster.ID, thisCluster.Name, oplogSizeStr, process.Hostname, strconv.Itoa(process.Port)}

					hours := 24

					var pmListOptPrimary = mongodbatlas.ProcessMeasurementListOptions{
						Granularity: "PT1H",
						// end on the previous top of the hour
						End:   time.Now().Format("2006-01-02T15") + ":00:00Z",
						Start: time.Now().Add(time.Hour*time.Duration(-hours)).Format("2006-01-02T15") + ":00:00Z",
						M:     []string{"OPLOG_MASTER_TIME", "OPLOG_RATE_GB_PER_HOUR"},
					}
					// These fields are available on the secondary
					// M:           []string{"OPLOG_MASTER_TIME", "OPLOG_RATE_GB_PER_HOUR", "OPLOG_SLAVE_LAG_MASTER_TIME", "OPLOG_MASTER_LAG_TIME_DIFF"},

					// Get process measurements for the process
					processMeasurements, _, err := client.ProcessMeasurements.List(context.Background(), project.ID, process.Hostname, process.Port, &pmListOptPrimary)
					if err != nil {
						// This is most likely "INVALID_METRIC_NAME" because the machine is in the free/shared tier and does not support oplog metrics
						if !strings.Contains(err.Error(), "INVALID_METRIC_NAME") {
							fmt.Printf("ProcessMeasurements.List returned error: %v\n", err)
						}
						continue
					}

					windowTotal := 0
					windowCount := 0
					windowMin := int(^uint(0) >> 1)
					windowMinHour := ""

					rateTotal := float32(0.0)
					rateCount := 0
					rateMax := float32(0.0)
					rateMaxHour := ""

					for _, measurement := range (*processMeasurements).Measurements {
						if d {
							fmt.Println("Measurement: ", measurement.Name, " (", measurement.Units, ")")
						}

						for _, dataPoint := range measurement.DataPoints {
							if dataPoint.Value != nil {
								hour := dataPoint.Timestamp[0:13]
								if measurement.Name == "OPLOG_MASTER_TIME" {
									if d {
										fmt.Println(dataPoint.Timestamp, int(*dataPoint.Value))
									}
									oplogWindowTimes[hour] = true
									window := int(*dataPoint.Value)
									windowTotal += window
									if window < windowMin {
										windowMin = window
										windowMinHour = hour
									}
									windowCount++
								} else {
									if d {
										fmt.Println(dataPoint.Timestamp, *dataPoint.Value)
									}
									oplogRateTimes[hour] = true
									rate := *dataPoint.Value
									rateTotal += rate
									if rate > rateMax {
										rateMax = rate
										rateMaxHour = hour
									}
									rateCount++
								}
							}
						}
					}

					fields := []string{}
					fields = append(projectFields, processFields...)

					if windowCount > 0 {
						windowMinHrs := float64(windowMin) / 3600.0
						fields = append(fields, strconv.FormatFloat(windowMinHrs, 'f', 2, 32))
						fields = append(fields, windowMinHour)
						windowAvg := float64(windowTotal) / float64(windowCount) / 3600.0
						fields = append(fields, strconv.FormatFloat(windowAvg, 'f', 2, 32))
					} else {
						fields = append(fields, "", "", "")
					}

					if rateCount > 0 {
						fields = append(fields, strconv.FormatFloat(float64(rateMax), 'f', 6, 32))
						fields = append(fields, rateMaxHour)
						rateAvg := float64(rateTotal) / float64(rateCount)
						fields = append(fields, strconv.FormatFloat(rateAvg, 'f', 6, 32))
					} else {
						fields = append(fields, "", "")
					}

					fmt.Println(strings.Join(fields, ","))

					if d {
						fmt.Println("")
					}
				}
			}
		}
	}
}
