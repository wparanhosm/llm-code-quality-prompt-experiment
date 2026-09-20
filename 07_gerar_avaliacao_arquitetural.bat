@echo off
echo ========================================================
echo ETAPA 7: Avaliacao arquitetural
echo ========================================================
wsl bash -lc "cd /mnt/c/Users/walte/workspace/llm-code-quality-prompt-experiment && python3 scripts/generate_architecture_report.py"
pause
