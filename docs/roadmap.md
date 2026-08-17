# Balemoh Roadmap

Balemoh é uma homepage para homelabs. O produto descobre recursos de forma semiautomática, apresenta candidatos numa área de staging e deixa o usuário decidir o que aparece na homepage.

## Princípios

1. **Descoberta não publica automaticamente.** Todo recurso descoberto começa como candidato.
2. **Adapters são somente leitura por padrão.** Docker, Kubernetes e extensões observam a plataforma; não alteram workload, ingress, service ou route.
3. **Identidade é estável.** O mesmo recurso precisa reaparecer como o mesmo candidato entre sincronizações.
4. **Evidência vence suposição.** Porta publicada não é hostname exato. URL só aparece quando uma fonte ou extensão a observou.
5. **Fonte fica separada do domínio.** O catálogo não conhece Docker SDK, Kubernetes client, RBAC ou socket.
6. **Estado do usuário sobrevive à descoberta.** Pin permanece quando metadata ou endpoints são atualizados.
7. **API primeiro.** Contrato OpenAPI, casos de uso e persistência vêm antes da UI Goshtoso.

## Fases

### Fase 0 — Fundação API e catálogo inicial

Status: em execução.

- contrato OpenAPI para staging, homepage e sincronização;
- domínio `Candidate`, fonte, recurso e endpoint;
- port `Discoverer` para conectores futuros;
- SQLite/sqlc para candidatos, endpoints e `pinned_at`;
- pin/unpin idempotente;
- documentação e testes de contrato, aplicação, storage e HTTP.

Fora desta fase: acesso real a Docker/Kubernetes e UI.

### Fase 1 — Discovery Docker read-only

- adapter para Docker Engine via socket ou endpoint configurado;
- identidade estável do host e dos containers;
- leitura de labels, nomes, estado e portas publicadas;
- normalização de portas como observações incompletas;
- limites explícitos para socket, timeout e exposição de metadata;
- testes com fixtures e cliente fake, sem exigir daemon no CI.

### Fase 2 — Extensões de rota Docker

- contrato de extensão para resolver endpoint externo;
- primeiro adapter Traefik, usando labels/configuração observável;
- provenance da URL (`traefik`, `docker.port`, etc.);
- conflito de múltiplas URLs e confiança visível no catálogo;
- nenhum hostname inferido silenciosamente.

### Fase 3 — Discovery Kubernetes read-only

- adapter com client Kubernetes e configuração de contexto/endpoint;
- RBAC mínimo para ler pods, services, ingresses, Gateway API `HTTPRoutes` e recursos necessários;
- identidade por cluster UID + namespace + kind + name;
- associação entre workload, service e route;
- hostname/path exatos vindos de Ingress ou HTTPRoute;
- tratamento de namespaces, permissões parciais e recursos removidos.

### Fase 4 — Reconciliação e operação de discovery

- sincronização periódica e execução manual;
- status por fonte, duração, último sucesso e erro sanitizado;
- stale candidates e política de retenção;
- deduplicação entre fontes;
- histórico de observações quando necessário;
- métricas, logs estruturados e limites de custo.

### Fase 5 — UI Goshtoso

- tela de staging com filtros por fonte, namespace e estado de endpoint;
- preview de card antes do pin;
- pin/unpin e edição de campos permitidos pelo usuário;
- homepage organizada por grupos/tags;
- indicação clara de endpoint exato versus observação incompleta;
- acessibilidade e composição genérica via componentes Goshtoso.

### Fase 6 — Segurança e distribuição

- autenticação e autorização da API/UI;
- proteção de segredos e socket Docker;
- política de allowlist para hosts/URLs exibidos;
- auditoria de decisões de pin e alterações manuais;
- empacotamento, deployment e backups somente após gates próprios.

## Critério de progresso

Cada fase termina quando tem contrato documentado, implementação isolada, testes reproduzíveis e limites de segurança explícitos. Verde técnico não autoriza merge, release, deploy ou publicação.

