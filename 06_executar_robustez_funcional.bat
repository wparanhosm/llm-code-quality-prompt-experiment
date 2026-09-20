@echo off
echo ========================================================
echo ETAPA 6: Casos negativos e concorrencia
echo ========================================================
wsl bash -lc "cd /mnt/c/Users/walte/workspace/llm-code-quality-prompt-experiment && python3 scripts/run_functional_robustness.py"
pause
