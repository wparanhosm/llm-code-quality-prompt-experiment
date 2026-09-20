@echo off
echo ========================================================
echo INICIANDO TODAS AS ETAPAS DO BENCHMARK EM SEQUENCIA
echo ========================================================

echo.
echo Executando Etapa 1...
call 01_classificar_snapshots.bat

echo.
echo Executando Etapa 2...
call 02_executar_benchmark.bat

echo.
echo Executando Etapa 3...
call 03_executar_benchmark_seguranca.bat

echo.
echo Executando Etapa 4...
call 04_analisar_experimentos.bat

echo.
echo ========================================================
echo TODOS OS SCRIPTS FINALIZADOS COM SUCESSO!
echo ========================================================
pause
