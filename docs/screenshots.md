# Screenshot Checklist

Use this guide to collect minimal proof screenshots for the project README and resume links.

## Folder structure

Create these folders in repo root:

```bash
mkdir -p docs/assets/screenshots
```

Suggested filenames:
- `docs/assets/screenshots/01-k8s-pods.png`
- `docs/assets/screenshots/02-aws-test-pass.png`
- `docs/assets/screenshots/03-query-campaign.png`
- `docs/assets/screenshots/04-s3-prefix-counts.png`
- `docs/assets/screenshots/05-k9s-runtime.png`
- `docs/assets/screenshots/06-k9s-alt.png` (optional)

Current files already copied:
- `01-k8s-pods.png`
- `02-aws-test-pass.png`
- `03-query-campaign.png`
- `04-s3-prefix-counts.png`
- `05-k9s-runtime.png`
- `06-k9s-alt.png`

## 1) Kubernetes healthy pods

Command:

```bash
kubectl -n event-pipeline get pods -o wide
```

Take screenshot as: `01-k8s-pods.png`

## 2) AWS test script success

Keep both port-forwards running in separate terminals:

```bash
kubectl -n event-pipeline port-forward svc/collector-service 3000:80
kubectl -n event-pipeline port-forward svc/query-service 3002:80
```

Then run in another terminal:

```bash
./test_aws.sh \
  --collector-url http://localhost:3000 \
  --query-url http://localhost:3002 \
  --strict-s3-check true
```

Take screenshot of terminal at final success output as: `02-aws-test-pass.png`

## 3) Query response by campaign id

Use campaign id from test output, then run:

```bash
CAMPAIGN_ID="<paste-campaign-id>"
curl -s "http://localhost:3002/campaigns/${CAMPAIGN_ID}" | jq
```

If `jq` is unavailable:

```bash
curl -s "http://localhost:3002/campaigns/${CAMPAIGN_ID}"
```

Take screenshot as: `03-query-campaign.png`

## 4) S3 archive evidence (optional but recommended)

```bash
S3_BUCKET=$(kubectl -n event-pipeline get configmap event-pipeline-config -o jsonpath='{.data.S3_BUCKET}')
aws s3api list-objects-v2 --region ap-south-1 --bucket "$S3_BUCKET" --prefix raw/ --query 'length(Contents)' --output text
aws s3api list-objects-v2 --region ap-south-1 --bucket "$S3_BUCKET" --prefix duplicate/ --query 'length(Contents)' --output text
aws s3api list-objects-v2 --region ap-south-1 --bucket "$S3_BUCKET" --prefix fraud/ --query 'length(Contents)' --output text
aws s3api list-objects-v2 --region ap-south-1 --bucket "$S3_BUCKET" --prefix accepted/ --query 'length(Contents)' --output text
```

Take screenshot as: `04-s3-prefix-counts.png`

## Suggested README embed section

```markdown
## Runtime Proof

![Kubernetes Pods](docs/assets/screenshots/01-k8s-pods.png)
![AWS Test Pass](docs/assets/screenshots/02-aws-test-pass.png)
![Query Campaign Response](docs/assets/screenshots/03-query-campaign.png)
![S3 Archive Prefix Counts](docs/assets/screenshots/04-s3-prefix-counts.png)
![K9s Runtime View](docs/assets/screenshots/05-k9s-runtime.png)
```
