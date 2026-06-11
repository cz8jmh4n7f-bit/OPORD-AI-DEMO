// Package pricing produces rough monthly cost estimates for OPORD resources.
// These are deliberately approximate (a static table + simple formulas) - enough
// for FinOps visibility and "what does this environment cost?", not billing.
package pricing

import "github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"

// instanceMonthlyUSD is a rough on-demand monthly price per instance/class
// (~730 h/mo, us-east-1-ish). Extend as needed.
var instanceMonthlyUSD = map[string]float64{
	// EC2
	"t3.micro": 7.5, "t3.small": 15, "t3.medium": 30, "t3.large": 60, "t3.xlarge": 120,
	"m5.large": 70, "m5.xlarge": 140, "c5.large": 62, "c5.xlarge": 124,
	// RDS
	"db.t3.micro": 13, "db.t3.small": 26, "db.t3.medium": 52, "db.m5.large": 125,
}

const (
	vcpuMonthlyUSD   = 20.0 // nominal per-vCPU/mo (used when there's no instance type)
	ramGBMonthlyUSD  = 2.5  // nominal per-GB-RAM/mo
	diskGBMonthlyUSD = 0.08 // gp3-ish per-GB/mo
)

func instance(class string) (float64, bool) {
	v, ok := instanceMonthlyUSD[class]
	return v, ok
}

// nodeMonthly estimates one node's monthly cost from cpu/mem/disk.
func nodeMonthly(cpu, memMB, diskGB int) float64 {
	memGB := float64(memMB) / 1024.0
	return float64(cpu)*vcpuMonthlyUSD + memGB*ramGBMonthlyUSD + float64(diskGB)*diskGBMonthlyUSD
}

// VM estimates a standalone-VM resource's monthly cost.
func VM(spec models.VMSpec) float64 {
	count := spec.Count
	if count < 1 {
		count = 1
	}
	var per float64
	if v, ok := instance(spec.InstanceType); ok {
		per = v + float64(spec.DiskGB)*diskGBMonthlyUSD
	} else {
		per = nodeMonthly(spec.CPU, spec.MemoryMB, spec.DiskGB)
	}
	return per * float64(count)
}

// Database estimates a managed-database resource's monthly cost.
func Database(spec models.DatabaseSpec) float64 {
	per, ok := instance(spec.InstanceClass)
	if !ok {
		per = 13 // default to db.t3.micro-ish
	}
	return per + float64(spec.StorageGB)*diskGBMonthlyUSD
}

// S3 estimates a small object-storage bucket. The storage component assumes a
// modest 100 GB baseline because OPORD does not yet ingest object bytes.
func S3(spec models.S3Spec) float64 {
	monthly := 100 * 0.023
	if spec.Versioning {
		monthly += 1.0
	}
	if spec.KMSKeyARN != "" {
		monthly += 1.0
	}
	return monthly
}

// Queue estimates a low-volume managed queue and its optional DLQ/KMS overhead.
func Queue(spec models.QueueSpec) float64 {
	monthly := 0.40
	if spec.DLQEnabled {
		monthly += 0.20
	}
	if spec.KMSKeyARN != "" {
		monthly += 1.0
	}
	return monthly
}

// Secret estimates the fixed monthly charge for a managed secret.
func Secret(spec models.SecretSpec) float64 {
	monthly := 0.40
	if spec.KMSKeyARN != "" {
		monthly += 1.0
	}
	if spec.RotationLambdaARN != "" {
		monthly += 0.25
	}
	return monthly
}

// Function estimates a small serverless function. Invocation data comes later
// from FOCUS; until then memory is the only meaningful static signal.
func Function(spec models.FunctionSpec) float64 {
	mem := spec.MemoryMB
	if mem <= 0 {
		mem = 128
	}
	return 0.25 + (float64(mem) / 1024.0 * 0.30)
}

// Table estimates a DynamoDB/Cosmos-like table from billing mode.
func Table(spec models.TableSpec) float64 {
	switch spec.BillingMode {
	case "PROVISIONED":
		read := spec.ReadCapacity
		write := spec.WriteCapacity
		if read <= 0 {
			read = 1
		}
		if write <= 0 {
			write = 1
		}
		return float64(read)*0.25 + float64(write)*1.25
	default:
		return 1.25
	}
}

// Stack has no predictable module shape, so OPORD uses a placeholder to keep
// it visible in FinOps guardrails without pretending to know exact spend.
func Stack() float64 { return 0 }

// Cluster estimates a Kubernetes cluster's monthly cost (sum of its nodes).
func Cluster(spec models.ClusterSpec) float64 {
	cp := spec.ControlPlane
	wk := spec.Workers
	cpCost := float64(cp.Count) * nodeMonthly(cp.Specs.CPU, cp.Specs.MemoryMB, cp.Specs.DiskGB)
	wkCost := float64(wk.Count) * nodeMonthly(wk.Specs.CPU, wk.Specs.MemoryMB, wk.Specs.DiskGB)
	return cpCost + wkCost
}
