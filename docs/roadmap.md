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

Status: concluída neste branch.

- contrato OpenAPI para staging, homepage e sincronização;
- domínio `Candidate`, fonte, recurso e endpoint;
- port `Discoverer` para conectores futuros;
- SQLite/sqlc para candidatos, endpoints, imagens e `pinned_at`;
- pin/unpin idempotente;
- documentação e testes de contrato, aplicação, storage e HTTP.

Fora desta fase: acesso real a Docker e UI.

### Fase 1 — Discovery Docker read-only

Status: base implementada neste branch para a API Docker-compatible de Docker/Podman.

- adapter para Docker Engine via socket ou endpoint configurado;
- identidade estável do host e dos containers;
- leitura de nomes, estado, imagens e portas publicadas;
- agrupamento de Compose services pelos labels canônicos, incluindo `podman-compose`;
- portas publicadas normalizadas com o IP/interface do host como observações incompletas;
- execução read-only sobre containers em execução;
- endpoint de socket explícito e metadata limitada a evidências de runtime/Compose;
- testes com fixtures e cliente fake, sem exigir daemon no CI.

Fora desta base: timeout configurável, resolução de hostname e informer/extensão Traefik.

### Fase 2 — Extensões de rota Docker

- contrato de extensão para resolver endpoint externo;
- primeiro adapter Traefik, usando labels/configuração observável;
- provenance da URL (`traefik`, `docker.port`, etc.);
- conflito de múltiplas URLs e confiança visível no catálogo;
- nenhum hostname inferido silenciosamente.

### Fase 3 — Discovery Kubernetes read-only

Status: em execução.

- adapter read-only com `client-go` typed para Pods/Services/Ingresses e dynamic client para Gateway API `HTTPRoutes`;
- configuração in-cluster explícita por `BALEMOH_KUBERNETES_ENABLED`, source ID estável e namespace;
- RBAC mínimo para ler Pods, Services, Ingresses e `HTTPRoutes`;
- identidade por source ID + namespace + kind + name;
- imagens de init containers, containers e ephemeral containers como evidência estruturada do Pod;
- hostname/path vindos de Ingress ou HTTPRoute; HTTPRoute usa URL scheme-relative porque o listener pode ser HTTP ou HTTPS;
- resolução preferencial `HTTPRoute/Ingress -> Service -> Pod`; sem rotas, fallback `Service -> Pod`;
- Pods órfãos ficam fora do staging; Services sem selector, `ExternalName` ou endereço externo continuam elegíveis;
- namespaces, CRD HTTPRoute ausente e erros de permissão tratados sem mutação;
- desenvolvimento local reproduzível via DevSpace sobre KinD ou vCluster in Docker (vind).

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
