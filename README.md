# slamdunk

Cloud Storage Bucket Permissions Auditor

## What is it?

`slamdunk` aids webapp hackers audit cloud storage bucket solutions (currently supported is AWS S3)
to find vulnerabilities and leaks that can be disclosed.

## How does it work?

`slamdunk` comprises of a _resolver_ and the main _auditor_. Both work in the following manner:

* The __resolver__ consumes URL(s), say generated by subdomain enumeration, and runs a set of heuristics
to try to figure out the unique bucket name identifier for it. This is useful for asset discovery
across an unknown domain under test, and results can be re-used for auditing.

* The __auditor__ consumes bucket name(s) and runs a supported set of actions from a playbook
(see `playbook.go`) to identify what permissiosn are available, and can potentially be misused for
privilege escalation or information leaking.

## What's in the Playbook?

WIP

## License

[MIT]()
