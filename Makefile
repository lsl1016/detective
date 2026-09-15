.PHONY: test compile-cases server evaluator selfcheck smoke bootstrap clean-runs

test:
	go test ./...

compile-cases:
	python3 scripts/compile_cases.py

server:
	go run ./caseserver

evaluator:
	go run ./evaluator -h

selfcheck:
	go run ./evaluator -mode selfcheck

smoke:
	bash scripts/smoke.sh

bootstrap:
	bash bootstrap.sh

clean-runs:
	rm -f eval/runs/*.jsonl eval/runs/*.json eval/runs/*.txt

crossquiz-master:
	go run ./evaluator -mode crossquiz -case CASE-015 -answers eval/templates/CASE-015.answers.json

master-eval:
	go run ./evaluator -mode master -case CASE-015 -master-verdict eval/templates/master_verdict.json
