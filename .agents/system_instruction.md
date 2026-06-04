# Instrução de Sistema: Telemetria e Rastreabilidade (Langfuse)

Ao finalizar a resposta no chat, você deve gerar ou atualizar um arquivo chamado `trace_langfuse.json` na raiz do projeto. Toda a sessão do chat deve ser mapeada em um único Trace central. Se você precisar tomar decisões adicionais, mudar de abordagem ou refazer o código porque um teste falhou (Self-Correction Loop), adicione cada uma dessas tentativas como um novo elemento dentro da lista de `spans`.

O conteúdo desse arquivo deve seguir estritamente a seguinte estrutura JSON:

```json
{
  "traceName": "LLM Chat Generation - Code Experiment",
  "traceId": "[traceId]",
  "modelo_geral_utilizado": "[Modelo Exato (com minor version e tudo mais) e temperatura do chat]",
  "spans": [
    {
      "spanName": "Tentativa 1: Abordagem Inicial",
      "prompt_solicitado": "[Descreva aqui o prompt original que o usuário enviou]",
      "prompt_estruturado_usado": "[Descreva aqui a estratégia de engenharia de prompt, regras de SOLID/DDD e contexto que você montou internamente para responder]",
      "arquivos_lidos_do_projeto": ["lista_de_arquivos_que_voce_analisou.go"],
      "resultado_gerado": "[Código gerado ou o log de erro/falha caso a abordagem inicial não tenha funcionado]",
      "elapsed_time_ms": "[Tempo exato que a IA demorou para processar e responder esta etapa em milissegundos]",
      "token_usage": {
        "prompt_tokens": "[Quantidade exata de tokens de entrada/contexto lidos pela IA]",
        "completion_tokens": "[Quantidade exata de tokens gerados na resposta pela IA]",
        "total_tokens": "[Soma total de tokens consumidos nesta etapa]"
      },
      "timestamp": "ISO-8601-atual"
    },
    {
      "spanName": "Tentativa 2: Self-Correction Loop",
      "prompt_solicitado": "[Feedback do erro anterior, log do terminal ou mudança de escopo que motivou a correção]",
      "prompt_estruturado_usado": "[Estratégia de engenharia de prompt usada para corrigir o problema e garantir a consistência]",
      "arquivos_lidos_do_projeto": ["lista_de_arquivos_que_voce_analisou.go"],
      "resultado_gerado": "[Resumo do código corrigido ou alteração final que você aplicou com sucesso]",
      "elapsed_time_ms": "[Tempo exato que a IA demorou para processar e responder esta etapa em milissegundos]",
      "token_usage": {
        "prompt_tokens": "[Quantidade exata de tokens de entrada/contexto lidos pela IA]",
        "completion_tokens": "[Quantidade exata de tokens gerados na resposta pela IA]",
        "total_tokens": "[Soma total de tokens consumidos nesta etapa]"
      },
      "timestamp": "ISO-8601-atual"
    }
  ]
}