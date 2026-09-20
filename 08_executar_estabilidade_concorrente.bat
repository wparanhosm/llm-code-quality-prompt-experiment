@echo off
echo ========================================================
echo ETAPA 8: Estabilidade concorrente antes do K6
echo ========================================================
wsl bash -lc "cd /mnt/c/Users/walte/workspace/llm-code-quality-prompt-experiment && python3 scripts/run_concurrency_stability.py"
pause
