@echo off
setlocal
echo ETAPA 9: Carga K6 de um milhao de requisicoes por backend elegivel
wsl bash -lc "cd /mnt/c/Users/walte/workspace/llm-code-quality-prompt-experiment && python3 scripts/run_k6_benchmark.py"
if errorlevel 1 exit /b %errorlevel%
endlocal
