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
