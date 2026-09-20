@echo off
echo ========================================================
echo ETAPA 5: Testes funcionais e reconciliacao de saldos
echo ========================================================
wsl bash -lc "cd /mnt/c/Users/walte/workspace/llm-code-quality-prompt-experiment && python3 scripts/run_functional_benchmark.py"
pause
