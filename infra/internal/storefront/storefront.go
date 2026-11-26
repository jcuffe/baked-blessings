// Package storefront hosts the containerized storefront on AWS: an ECR repository for the
// image CI builds, a Lambda that runs it (through the Lambda Web Adapter baked into the
// image), and a CloudFront distribution in front whose *.cloudfront.net address is the
// public link until a domain is bought. A GitHub OIDC role lets CI push images and roll
// the Lambda without long-lived AWS keys.
//
// The same image runs unchanged on Fargate or Kubernetes; only this package changes when
// the storefront moves there.
package storefront

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cloudfront"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ecr"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/jcuffe/baked-blessings/infra/internal/config"
)

// AWS-managed CloudFront policies (stable IDs, the same in every account).
const (
	cachingDisabled             = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
	cachingOptimized            = "658327ea-f89e-4fab-a63d-7e88639e58f6"
	allViewerExceptHostHeader   = "b689b0a8-53d0-40ab-baf2-68738e2966ac"
	githubOIDCHost              = "token.actions.githubusercontent.com"
	originVerifyHeader          = "x-origin-verify"
	keepImages                  = 10
	lambdaLogRetentionDays      = 14
	lambdaTimeoutSeconds        = 30
	lambdaArchitecture          = "arm64"
	cloudFrontPriceClass        = "PriceClass_100"
	cloudFrontOriginReadTimeout = 30
)

// forwardHost copies the viewer's Host into x-forwarded-host. CloudFront has to send the
// Lambda URL's own host to the origin, and the app needs the public one to build links.
const forwardHost = `function handler(event) {
  var request = event.request;
  request.headers['x-forwarded-host'] = {value: request.headers.host.value};
  return request;
}`

// Storefront is what the rest of the program and CI need from this package.
type Storefront struct {
	RepositoryURL pulumi.StringOutput
	CIRoleARN     pulumi.StringOutput
	// FunctionName and URL are empty until the stack has a storefrontImageTag.
	FunctionName pulumi.StringOutput
	URL          pulumi.StringOutput
}

// New creates the storefront's AWS resources for the stack.
func New(ctx *pulumi.Context, cfg *config.Config) (*Storefront, error) {
	sf := cfg.Storefront
	name := cfg.ResourceName("storefront")

	repo, err := ecr.NewRepository(ctx, "storefront", &ecr.RepositoryArgs{
		Name: pulumi.String("baked-blessings/" + name),
		// Mutable, so re-running CI on an older commit (a rollback) can push its tag again.
		ImageTagMutability: pulumi.String("MUTABLE"),
		ImageScanningConfiguration: &ecr.RepositoryImageScanningConfigurationArgs{
			ScanOnPush: pulumi.Bool(true),
		},
	})
	if err != nil {
		return nil, err
	}

	// ECR storage is billed per GB-month, so keep only recent images.
	if _, err := ecr.NewLifecyclePolicy(ctx, "storefront", &ecr.LifecyclePolicyArgs{
		Repository: repo.Name,
		Policy: pulumi.String(mustJSON(map[string]any{
			"rules": []any{map[string]any{
				"rulePriority": 1,
				"description":  fmt.Sprintf("Keep the last %d images", keepImages),
				"selection": map[string]any{
					"tagStatus":   "any",
					"countType":   "imageCountMoreThan",
					"countNumber": keepImages,
				},
				"action": map[string]any{"type": "expire"},
			}},
		})),
	}); err != nil {
		return nil, err
	}

	out := &Storefront{
		RepositoryURL: repo.RepositoryUrl,
		FunctionName:  pulumi.String("").ToStringOutput(),
		URL:           pulumi.String("").ToStringOutput(),
	}

	var fn *lambda.Function
	if sf.ImageTag != "" {
		if fn, out.URL, err = serve(ctx, cfg, name, repo); err != nil {
			return nil, err
		}
		out.FunctionName = fn.Name
	}

	if out.CIRoleARN, err = ciRole(ctx, cfg, name, repo, fn); err != nil {
		return nil, err
	}
	return out, nil
}

// serve creates the Lambda, its function URL and the CloudFront distribution in front.
func serve(ctx *pulumi.Context, cfg *config.Config, name string, repo *ecr.Repository) (*lambda.Function, pulumi.StringOutput, error) {
	sf := cfg.Storefront
	none := pulumi.StringOutput{}

	role, err := iam.NewRole(ctx, "storefront-lambda", &iam.RoleArgs{
		Name:             pulumi.String(name + "-lambda"),
		AssumeRolePolicy: pulumi.String(assumeRole(map[string]any{"Service": "lambda.amazonaws.com"}, nil)),
	})
	if err != nil {
		return nil, none, err
	}
	if _, err := iam.NewRolePolicyAttachment(ctx, "storefront-lambda-logs", &iam.RolePolicyAttachmentArgs{
		Role:      role.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
	}); err != nil {
		return nil, none, err
	}

	logs, err := cloudwatch.NewLogGroup(ctx, "storefront", &cloudwatch.LogGroupArgs{
		Name:            pulumi.String("/aws/lambda/" + name),
		RetentionInDays: pulumi.Int(lambdaLogRetentionDays),
	})
	if err != nil {
		return nil, none, err
	}

	// Only CloudFront knows this value, so requests that skip the CDN are refused.
	originSecret, err := random.NewRandomPassword(ctx, "storefront-origin-secret", &random.RandomPasswordArgs{
		Length:  pulumi.Int(40),
		Special: pulumi.Bool(false),
	})
	if err != nil {
		return nil, none, err
	}

	env := pulumi.StringMap{
		"NODE_ENV":             pulumi.String("production"),
		"ORIGIN_VERIFY_SECRET": originSecret.Result,
	}
	for k, v := range sf.Env {
		env[k] = pulumi.String(v)
	}
	for k, v := range sf.Secrets {
		env[k] = v
	}

	fn, err := lambda.NewFunction(ctx, "storefront", &lambda.FunctionArgs{
		Name:          pulumi.String(name),
		Role:          role.Arn,
		PackageType:   pulumi.String("Image"),
		ImageUri:      pulumi.Sprintf("%s:%s", repo.RepositoryUrl, sf.ImageTag),
		Architectures: pulumi.StringArray{pulumi.String(lambdaArchitecture)},
		MemorySize:    pulumi.Int(sf.MemoryMB),
		Timeout:       pulumi.Int(lambdaTimeoutSeconds),
		Environment:   &lambda.FunctionEnvironmentArgs{Variables: env},
		LoggingConfig: &lambda.FunctionLoggingConfigArgs{
			LogFormat: pulumi.String("Text"),
			LogGroup:  logs.Name,
		},
	},
		// CI rolls new images onto the function; the tag in config is only the first one.
		pulumi.IgnoreChanges([]string{"imageUri"}),
	)
	if err != nil {
		return nil, none, err
	}

	fnURL, err := lambda.NewFunctionUrl(ctx, "storefront", &lambda.FunctionUrlArgs{
		FunctionName:      fn.Name,
		AuthorizationType: pulumi.String("NONE"),
		InvokeMode:        pulumi.String("RESPONSE_STREAM"),
	})
	if err != nil {
		return nil, none, err
	}

	// A public function URL needs both permissions. CloudFront can't sign POST bodies
	// for an IAM-authorized URL (cart forms post), so the origin secret guards it instead.
	if _, err := lambda.NewPermission(ctx, "storefront-url-invoke", &lambda.PermissionArgs{
		Function:            fn.Name,
		Action:              pulumi.String("lambda:InvokeFunctionUrl"),
		Principal:           pulumi.String("*"),
		FunctionUrlAuthType: pulumi.String("NONE"),
	}); err != nil {
		return nil, none, err
	}
	if _, err := lambda.NewPermission(ctx, "storefront-url-function", &lambda.PermissionArgs{
		Function:              fn.Name,
		Action:                pulumi.String("lambda:InvokeFunction"),
		Principal:             pulumi.String("*"),
		InvokedViaFunctionUrl: pulumi.Bool(true),
	}); err != nil {
		return nil, none, err
	}

	hostFn, err := cloudfront.NewFunction(ctx, "storefront-forward-host", &cloudfront.FunctionArgs{
		Name:    pulumi.String(name + "-forward-host"),
		Runtime: pulumi.String("cloudfront-js-2.0"),
		Code:    pulumi.String(forwardHost),
		Publish: pulumi.Bool(true),
	})
	if err != nil {
		return nil, none, err
	}

	originHost := fnURL.FunctionUrl.ApplyT(func(u string) string {
		return strings.TrimSuffix(strings.TrimPrefix(u, "https://"), "/")
	}).(pulumi.StringOutput)
	viewerRequest := cloudfront.DistributionDefaultCacheBehaviorFunctionAssociationArray{
		&cloudfront.DistributionDefaultCacheBehaviorFunctionAssociationArgs{
			EventType:   pulumi.String("viewer-request"),
			FunctionArn: hostFn.Arn,
		},
	}
	allMethods := pulumi.ToStringArray([]string{"GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"})
	readMethods := pulumi.ToStringArray([]string{"GET", "HEAD"})

	cdn, err := cloudfront.NewDistribution(ctx, "storefront", &cloudfront.DistributionArgs{
		Comment:       pulumi.String("Baked Blessings storefront (" + cfg.Stack + ")"),
		Enabled:       pulumi.Bool(true),
		IsIpv6Enabled: pulumi.Bool(true),
		HttpVersion:   pulumi.String("http2and3"),
		PriceClass:    pulumi.String(cloudFrontPriceClass),
		Origins: cloudfront.DistributionOriginArray{
			&cloudfront.DistributionOriginArgs{
				OriginId:   pulumi.String("lambda"),
				DomainName: originHost,
				CustomOriginConfig: &cloudfront.DistributionOriginCustomOriginConfigArgs{
					HttpPort:             pulumi.Int(80),
					HttpsPort:            pulumi.Int(443),
					OriginProtocolPolicy: pulumi.String("https-only"),
					OriginSslProtocols:   pulumi.ToStringArray([]string{"TLSv1.2"}),
					OriginReadTimeout:    pulumi.Int(cloudFrontOriginReadTimeout),
				},
				CustomHeaders: cloudfront.DistributionOriginCustomHeaderArray{
					&cloudfront.DistributionOriginCustomHeaderArgs{
						Name:  pulumi.String(originVerifyHeader),
						Value: originSecret.Result,
					},
				},
			},
		},
		// Pages are rendered per request (carts, sessions); Hydrogen caches Shopify data itself.
		DefaultCacheBehavior: &cloudfront.DistributionDefaultCacheBehaviorArgs{
			TargetOriginId:        pulumi.String("lambda"),
			ViewerProtocolPolicy:  pulumi.String("redirect-to-https"),
			AllowedMethods:        allMethods,
			CachedMethods:         readMethods,
			Compress:              pulumi.Bool(true),
			CachePolicyId:         pulumi.String(cachingDisabled),
			OriginRequestPolicyId: pulumi.String(allViewerExceptHostHeader),
			FunctionAssociations:  viewerRequest,
		},
		// Hashed build assets never change, so the edge keeps them.
		OrderedCacheBehaviors: cloudfront.DistributionOrderedCacheBehaviorArray{
			&cloudfront.DistributionOrderedCacheBehaviorArgs{
				PathPattern:          pulumi.String("/assets/*"),
				TargetOriginId:       pulumi.String("lambda"),
				ViewerProtocolPolicy: pulumi.String("redirect-to-https"),
				AllowedMethods:       readMethods,
				CachedMethods:        readMethods,
				Compress:             pulumi.Bool(true),
				CachePolicyId:        pulumi.String(cachingOptimized),
			},
		},
		Restrictions: &cloudfront.DistributionRestrictionsArgs{
			GeoRestriction: &cloudfront.DistributionRestrictionsGeoRestrictionArgs{
				RestrictionType: pulumi.String("none"),
			},
		},
		ViewerCertificate: &cloudfront.DistributionViewerCertificateArgs{
			CloudfrontDefaultCertificate: pulumi.Bool(true),
		},
	})
	if err != nil {
		return nil, none, err
	}

	return fn, pulumi.Sprintf("https://%s", cdn.DomainName), nil
}

// ciRole lets the stack's GitHub Actions environment push images and roll the Lambda.
func ciRole(ctx *pulumi.Context, cfg *config.Config, name string, repo *ecr.Repository, fn *lambda.Function) (pulumi.StringOutput, error) {
	sf := cfg.Storefront
	none := pulumi.StringOutput{}
	if cfg.GitHubRepository == "" {
		return pulumi.String("").ToStringOutput(), nil
	}

	providerARN := pulumi.String(sf.GitHubOIDCProviderARN).ToStringOutput()
	if sf.GitHubOIDCProviderARN == "" {
		provider, err := iam.NewOpenIdConnectProvider(ctx, "github-actions", &iam.OpenIdConnectProviderArgs{
			Url:           pulumi.String("https://" + githubOIDCHost),
			ClientIdLists: pulumi.ToStringArray([]string{"sts.amazonaws.com"}),
			ThumbprintLists: pulumi.ToStringArray([]string{
				// AWS no longer checks this for GitHub, but the API still asks for one.
				"6938fd4d98bab03faadb97b34396831e3780aea1",
			}),
		})
		if err != nil {
			return none, err
		}
		providerARN = provider.Arn
	}

	trust := providerARN.ApplyT(func(arn string) string {
		return assumeRole(map[string]any{"Federated": arn}, map[string]any{
			"StringEquals": map[string]any{
				githubOIDCHost + ":aud": "sts.amazonaws.com",
				// Only jobs that run in this stack's environment (staging or prod).
				githubOIDCHost + ":sub": fmt.Sprintf("repo:%s:environment:%s", cfg.GitHubRepository, cfg.Stack),
			},
		})
	}).(pulumi.StringOutput)

	role, err := iam.NewRole(ctx, "storefront-ci", &iam.RoleArgs{
		Name:             pulumi.String(name + "-ci"),
		AssumeRolePolicy: trust,
	})
	if err != nil {
		return none, err
	}

	fnARN := pulumi.String("").ToStringOutput()
	if fn != nil {
		fnARN = fn.Arn
	}
	policy := pulumi.All(repo.Arn, fnARN).ApplyT(func(args []any) string {
		repoARN, functionARN := args[0].(string), args[1].(string)
		statements := []any{
			map[string]any{"Effect": "Allow", "Action": "ecr:GetAuthorizationToken", "Resource": "*"},
			map[string]any{
				"Effect": "Allow",
				"Action": []string{
					"ecr:BatchCheckLayerAvailability",
					"ecr:BatchGetImage",
					"ecr:CompleteLayerUpload",
					"ecr:GetDownloadUrlForLayer",
					"ecr:InitiateLayerUpload",
					"ecr:PutImage",
					"ecr:UploadLayerPart",
				},
				"Resource": repoARN,
			},
		}
		if functionARN != "" {
			statements = append(statements, map[string]any{
				"Effect":   "Allow",
				"Action":   []string{"lambda:UpdateFunctionCode", "lambda:GetFunction", "lambda:GetFunctionConfiguration"},
				"Resource": functionARN,
			})
		}
		return mustJSON(map[string]any{"Version": "2012-10-17", "Statement": statements})
	}).(pulumi.StringOutput)

	if _, err := iam.NewRolePolicy(ctx, "storefront-ci", &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: policy,
	}); err != nil {
		return none, err
	}
	return role.Arn, nil
}

func assumeRole(principal map[string]any, condition map[string]any) string {
	statement := map[string]any{
		"Effect":    "Allow",
		"Principal": principal,
		"Action":    "sts:AssumeRole",
	}
	if _, federated := principal["Federated"]; federated {
		statement["Action"] = "sts:AssumeRoleWithWebIdentity"
	}
	if condition != nil {
		statement["Condition"] = condition
	}
	return mustJSON(map[string]any{"Version": "2012-10-17", "Statement": []any{statement}})
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
