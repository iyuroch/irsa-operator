package aws

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// IIAMClient provides wrapper interface for mocks
type IIAMClient interface {
	CreatePolicy(ctx context.Context, params *iam.CreatePolicyInput, optFns ...func(*iam.Options)) (*iam.CreatePolicyOutput, error)
	DeletePolicy(ctx context.Context, params *iam.DeletePolicyInput, optFns ...func(*iam.Options)) (*iam.DeletePolicyOutput, error)
	GetPolicy(ctx context.Context, params *iam.GetPolicyInput, optFns ...func(*iam.Options)) (*iam.GetPolicyOutput, error)
	DeletePolicyVersion(ctx context.Context, params *iam.DeletePolicyVersionInput,
		optFns ...func(*iam.Options)) (*iam.DeletePolicyVersionOutput, error)
	CreatePolicyVersion(ctx context.Context, params *iam.CreatePolicyVersionInput,
		optFns ...func(*iam.Options)) (*iam.CreatePolicyVersionOutput, error)
	ListPolicyVersions(ctx context.Context, params *iam.ListPolicyVersionsInput,
		optFns ...func(*iam.Options)) (*iam.ListPolicyVersionsOutput, error)
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)
	AttachRolePolicy(ctx context.Context, params *iam.AttachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.AttachRolePolicyOutput, error)
	DeleteRole(ctx context.Context, params *iam.DeleteRoleInput, optFns ...func(*iam.Options)) (*iam.DeleteRoleOutput, error)
	DetachRolePolicy(ctx context.Context, params *iam.DetachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.DetachRolePolicyOutput, error)
}

func getRegion() string {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		return "us-east-1"
	}
	return region
}

// NewIAMClient gets system wide iam client for aws
func NewIAMClient() (IIAMClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	cfg.Region = getRegion()

	return iam.NewFromConfig(cfg), nil
}

var cacheAccountId string

// GetAccountId gets account id, cache results and return if cache exists
func GetAccountId() (string, error) {
	// TODO: consider moving it somewhere or passing to IAM during initialisation
	if cacheAccountId != "" {
		return cacheAccountId, nil
	}

	ctx := context.TODO()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}

	cfg.Region = getRegion()

	client := sts.NewFromConfig(cfg)
	callerIdentity, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	cacheAccountId = *callerIdentity.Account
	return cacheAccountId, nil
}
