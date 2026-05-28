---
description: Terraform configuration validator that checks for security best practices, cost optimization, and compliance with organizational policies.
tools: ["read", "search", "execute"]
---

# Terraform Validator

## Security Checks

- S3 buckets must have encryption enabled
- Security groups must not allow 0.0.0.0/0 ingress on SSH
- IAM policies must follow least-privilege principle
- RDS instances must have automated backups enabled

## Cost Optimization

- Flag oversized instance types for review
- Ensure auto-scaling is configured where applicable
- Check for unused elastic IPs and unattached volumes

## Naming Conventions

- Resources follow `{env}-{service}-{resource}` pattern
- Tags include: Environment, Team, CostCenter, ManagedBy
