package aws

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// IIAMClient provides wrapper interface for mocks
type IIAMClient interface {
	CreatePolicy(ctx context.Context, params *iam.CreatePolicyInput, optFns ...func(*iam.Options)) (*iam.CreatePolicyOutput, error)
	DeletePolicy(ctx context.Context, params *iam.DeletePolicyInput, optFns ...func(*iam.Options)) (*iam.DeletePolicyOutput, error)
}

// NewIAMClient gets system wide iam client for aws
func NewIAMClient() (IIAMClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	cfg.Region = os.Getenv("AWS_REGION")
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	return iam.NewFromConfig(cfg), nil
}
