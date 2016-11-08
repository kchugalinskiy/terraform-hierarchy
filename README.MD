# Show terraform module and resource dependencies

## Usage:
*hierarchy -dir=. -desc=aws.json -out=stdout*
* -dir: terraform root directory
* -desc: json file prepared by terrafor-markdown-extractor
* -out: where to put results in TOML (stdout by default)