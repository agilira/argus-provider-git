# Argus Git Provider

Un provider GitOps completo per Argus che consente il caricamento e il monitoraggio di configurazioni da repository Git.

## Caratteristiche

### 🚀 Funzionalità Principali
- **Supporto Multi-Repository**: Funziona con GitHub, GitLab, Bitbucket e server Git self-hosted
- **Autenticazione Sicura**: Supporta token, chiavi SSH e autenticazione basic
- **Watch Intelligente**: Utilizza `git ls-remote` per un polling efficiente senza cloni completi
- **Cache Avanzata**: Cache intelligente per evitare re-download non necessari
- **Retry Robusto**: Sistema di retry con backoff esponenziale per operazioni di rete

### 🔒 Sicurezza
- **Validazione Path**: Protezione contro attacchi di path traversal
- **Sanitizzazione URL**: Prevenzione di attacchi SSRF e protocol confusion
- **Limiti Risorse**: Protezione DoS con limiti su file size, operazioni concorrenti e cache
- **Audit Completo**: Logging di sicurezza per monitoraggio e compliance

### ⚡ Performance
- **Zero Allocations**: Hot path ottimizzato senza allocazioni di memoria
- **Cloni Shallow**: Solo l'ultimo commit per ridurre traffico e tempo
- **Cache Multi-Layer**: Cache per autenticazione, metadati repository e configurazioni
- **Operazioni Concorrenti**: Supporto per operazioni parallele con limiti di sicurezza

## Installazione

```bash
go get github.com/agilira/argus-provider-git
```

## Utilizzo Base

### Configurazione Semplice

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    gitprovider "github.com/agilira/argus-provider-git"
)

func main() {
    // Crea il provider
    provider := gitprovider.GetProvider()
    defer provider.Close()
    
    ctx := context.Background()
    
    // URL per caricare config.json dal branch main
    configURL := "https://github.com/myorg/configs.git#config.json?ref=main"
    
    // Carica la configurazione
    config, err := provider.Load(ctx, configURL)
    if err != nil {
        log.Fatalf("Errore caricamento config: %v", err)
    }
    
    fmt.Printf("Configurazione caricata: %+v\n", config)
}
```

### Watch per Aggiornamenti Automatici

```go
func watchExample() {
    provider := gitprovider.GetProvider()
    defer provider.Close()
    
    ctx := context.Background()
    
    // URL con polling personalizzato ogni 30 secondi
    configURL := "https://github.com/myorg/configs.git#config.json?ref=main&poll=30s"
    
    // Inizia il watch
    configChan, err := provider.Watch(ctx, configURL)
    if err != nil {
        log.Fatalf("Errore watch: %v", err)
    }
    
    // Ascolta per cambiamenti
    for config := range configChan {
        fmt.Printf("Configurazione aggiornata: %+v\n", config)
        // Applica la nuova configurazione alla tua applicazione
        applyConfig(config)
    }
}
```

## Formati URL Supportati

### Struttura URL Base

```
<scheme>://<host>/<path>#<file>?<parameters>
```

### Schemi Supportati
- `https://` - HTTPS (raccomandato)
- `ssh://` - SSH
- `git://` - Git protocol
- `git+ssh://` - Git over SSH

### Esempi URL

#### GitHub
```bash
# Repository pubblico con JSON
https://github.com/user/repo.git#config.json?ref=main

# Repository privato con token
https://github.com/user/repo.git#configs/prod.yaml?ref=v1.0&auth=token:ghp_xxxxx

# Con SSH
ssh://git@github.com/user/repo.git#config.toml?ref=develop&auth=key:/path/to/key
```

#### GitLab
```bash
# Con token GitLab
https://gitlab.com/user/repo.git#config.json?auth=token:glpat_xxxxx

# Con autenticazione basic
https://gitlab.com/user/repo.git#config.yaml?auth=basic:username:password
```

#### Self-Hosted
```bash
# Server Git aziendale
https://git.company.com/team/configs.git#app.json?ref=production

# Con porta personalizzata
git://git.internal:9418/configs.git#config.toml?ref=staging
```

## Parametri URL

### File e Riferimenti
- `#<file>` - Path al file di configurazione nel repository
- `?file=<file>` - Alternativa al fragment per specificare il file
- `ref=<branch|tag|commit>` - Branch, tag o commit specifico (default: "main")
- `branch=<name>` - Alias per ref (per compatibilità)
- `tag=<name>` - Specifica un tag specifico
- `commit=<hash>` - Specifica un commit specifico

### Autenticazione
- `auth=token:<token>` - Token di accesso (GitHub/GitLab)
- `auth=basic:<user>:<pass>` - HTTP Basic Authentication
- `auth=key:<path>` - Chiave SSH privata
- `auth=ssh:<path>:<passphrase>` - Chiave SSH con passphrase

### Configurazione Watch
- `poll=<duration>` - Intervallo di polling (es: "30s", "5m", "1h")

## Formati di Configurazione Supportati

### JSON
```json
{
  "database": {
    "host": "localhost",
    "port": 5432
  },
  "features": {
    "caching": true
  }
}
```

### YAML
```yaml
database:
  host: localhost
  port: 5432
features:
  caching: true
```

### TOML
```toml
[database]
host = "localhost"
port = 5432

[features]
caching = true
```

## Autenticazione

### Token GitHub
```bash
# Crea un Personal Access Token su GitHub
# Scopes necessari: repo (per repo privati)
https://github.com/user/private-repo.git#config.json?auth=token:ghp_xxxxxxxxx
```

### Token GitLab
```bash
# Crea un Project Access Token su GitLab
# Scopes necessari: read_repository
https://gitlab.com/user/repo.git#config.json?auth=token:glpat-xxxxxxx
```

### Chiavi SSH
```bash
# Assicurati che la chiave SSH sia registrata nel tuo account Git
ssh://git@github.com/user/repo.git#config.json?auth=key:/home/user/.ssh/id_rsa

# Con passphrase
ssh://git@gitlab.com/user/repo.git#config.yaml?auth=ssh:/path/to/key:mypassphrase
```

### HTTP Basic Auth
```bash
# Per server Git che supportano basic auth
https://git.company.com/repo.git#config.json?auth=basic:username:password
```

## Configurazione Avanzata

### Limiti e Timeouts

Il provider ha limiti configurati per sicurezza e performance:

```go
const (
    maxConfigFileSize         = 5 * 1024 * 1024  // 5MB max per file
    maxConcurrentOperations   = 10               // Max operazioni parallele
    maxActiveWatches         = 5                // Max watch attivi
    defaultGitTimeout        = 60 * time.Second // Timeout operazioni Git
    minPollInterval          = 5 * time.Second  // Min intervallo polling
    maxPollInterval          = 10 * time.Minute // Max intervallo polling
)
```

### Cache Configuration

Il provider include un sistema di cache multi-layer:

- **Cache Autenticazione**: Cache per oggetti auth per evitare rigenerazione
- **Cache Metadati Repository**: Cache commit hash per ottimizzare il polling
- **Cache Configurazioni**: Cache configurazioni caricate per evitare re-download

```go
// Il provider usa cache intelligente con TTL di 10 minuti
// e capacità massima di 100 configurazioni
configCache: newConfigCache(100, 10*time.Minute)
```

## Monitoraggio e Metriche

### Metriche Disponibili

Il provider espone metriche dettagliate:

```go
provider := gitprovider.GetProvider()
metrics := provider.GetMetrics()

fmt.Printf("Cache hit rate: %.2f%%\n", metrics["cache_hit_rate"])
fmt.Printf("Total requests: %d\n", metrics["total_requests"])
fmt.Printf("Average load time: %.2fms\n", metrics["avg_load_time_ms"])
```

### Tipi di Metriche

#### Metriche di Richiesta
- `load_requests` - Totale chiamate Load()
- `watch_requests` - Totale chiamate Watch()
- `total_requests` - Totale richieste

#### Metriche di Cache
- `cache_hits` - Hit della cache
- `cache_misses` - Miss della cache
- `cache_hit_rate` - Percentuale hit rate
- `configs_cached` - Configurazioni in cache

#### Metriche di Performance
- `retry_attempts` - Tentativi di retry
- `failed_operations` - Operazioni fallite
- `avg_load_time_ms` - Tempo medio di caricamento
- `total_clone_time_ms` - Tempo totale per cloni
- `total_parse_time_ms` - Tempo totale per parsing

#### Metriche di Errore
- `network_errors` - Errori di rete
- `auth_errors` - Errori di autenticazione
- `parse_errors` - Errori di parsing
- `git_errors` - Errori Git

## Troubleshooting

### Problemi Comuni

#### 1. Errori di Autenticazione

**Problema**: `authentication failed` o `permission denied`

**Soluzioni**:
- Verifica che il token abbia i permessi corretti
- Per repository privati, assicurati che il token abbia scope `repo`
- Per chiavi SSH, verifica che sia registrata nell'account Git
- Testa l'autenticazione manualmente: `git ls-remote <repo-url>`

```bash
# Test manual con token GitHub
git ls-remote https://token:ghp_xxxxx@github.com/user/repo.git

# Test manual con SSH
git ls-remote git@github.com:user/repo.git
```

#### 2. File Non Trovato

**Problema**: `failed to read configuration file`

**Soluzioni**:
- Verifica che il file esista nel repository e branch specificato
- Controlla il path del file (case-sensitive)
- Assicurati che il branch/tag esista

```bash
# Verifica file nel repository
git ls-tree HEAD -- config.json

# Lista tutti i branch
git branch -r
```

#### 3. Errori di Formato

**Problema**: `failed to parse JSON/YAML/TOML configuration`

**Soluzioni**:
- Valida il formato del file di configurazione
- Usa un validator online per JSON/YAML/TOML
- Controlla encoding del file (deve essere UTF-8)

#### 4. Timeout

**Problema**: `timeout` durante le operazioni

**Soluzioni**:
- Verifica la connettività di rete al server Git
- Per repository grandi, considera l'uso di shallow clones (già abilitati)
- Aumenta timeout se necessario (configurabile)

#### 5. Limiti di Rate

**Problema**: `rate limit exceeded`

**Soluzioni**:
- Per GitHub, usa un token autenticato invece di accesso anonimo
- Aumenta l'intervallo di polling per le watch
- Considera l'uso di GitHub Apps per limiti più alti

### Debug e Logging

#### Abilitare Debug Logging

```go
// Per debug dettagliato, imposta handler di errore personalizzato
provider := gitprovider.GetProvider()
provider.ErrorHandler = func(err error, filepath string) {
    log.Printf("Git Provider Error [%s]: %v", filepath, err)
}
```

#### Monitorare Metriche

```go
// Stampa metriche periodicamente per debugging
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        metrics := provider.GetMetrics()
        log.Printf("Git Provider Metrics: %+v", metrics)
    }
}()
```

### Performance Tuning

#### Per Repository Grandi

```bash
# Usa branch specifici invece di HEAD
?ref=production

# Polling meno frequente
?poll=5m
```

#### Per Molte Configurazioni

```bash
# Usa cache TTL più lunghi in ambiente di produzione
# (configurabile nel codice del provider)
```

#### Per Alta Disponibilità

- Usa più provider in load balancing
- Implementa fallback locali per configurazioni critiche
- Monitora metriche di failure e retry

## Esempi Avanzati

### Integrazione con Argus

```go
package main

import (
    "context"
    "time"
    
    "github.com/agilira/argus"
    gitprovider "github.com/agilira/argus-provider-git"
)

func main() {
    // Configura Argus con supporto remoto Git
    watcher := argus.New(argus.Config{
        PollInterval: 30 * time.Second,
        Remote: argus.RemoteConfig{
            Enabled:     true,
            PrimaryURL:  "https://github.com/myorg/configs.git#app.json?ref=production&auth=token:xxx",
            FallbackPath: "./config/fallback.json",
            SyncInterval: 60 * time.Second,
            Timeout:     30 * time.Second,
        },
    })
    
    // Registra il provider Git
    // (questo sarà fatto automaticamente quando si importa il package)
    
    watcher.Start()
    defer watcher.Stop()
    
    // Il resto della tua applicazione...
}
```

### Multi-Environment Setup

```go
type ConfigManager struct {
    provider gitprovider.RemoteConfigProvider
    envConfigs map[string]string
}

func NewConfigManager() *ConfigManager {
    return &ConfigManager{
        provider: gitprovider.GetProvider(),
        envConfigs: map[string]string{
            "dev":  "https://github.com/myorg/configs.git#dev.json?ref=develop",
            "prod": "https://github.com/myorg/configs.git#prod.json?ref=production&auth=token:xxx",
        },
    }
}

func (cm *ConfigManager) LoadConfig(env string) (map[string]interface{}, error) {
    configURL, exists := cm.envConfigs[env]
    if !exists {
        return nil, fmt.Errorf("unknown environment: %s", env)
    }
    
    return cm.provider.Load(context.Background(), configURL)
}
```

### Circuit Breaker Pattern

```go
type CircuitBreakerGitProvider struct {
    provider gitprovider.RemoteConfigProvider
    failureCount int
    lastFailure  time.Time
    threshold    int
    timeout      time.Duration
}

func (cb *CircuitBreakerGitProvider) Load(ctx context.Context, url string) (map[string]interface{}, error) {
    // Implementa circuit breaker logic
    if cb.isCircuitOpen() {
        return nil, fmt.Errorf("circuit breaker is open")
    }
    
    config, err := cb.provider.Load(ctx, url)
    if err != nil {
        cb.recordFailure()
        return nil, err
    }
    
    cb.recordSuccess()
    return config, nil
}
```

## Sicurezza

### Best Practices

1. **Usa sempre HTTPS** per repository pubblici
2. **Limita scope dei token** al minimo necessario
3. **Ruota i token regolarmente**
4. **Non committare credenziali** nei repository
5. **Usa variabili d'ambiente** per token e credentials
6. **Monitora accessi e audit logs**

### Variabili d'Ambiente

```bash
# Esempio configurazione sicura
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxx"
export GITLAB_TOKEN="glpat-xxxxxxxxxxxxxxx"
export SSH_KEY_PATH="/secure/path/to/key"

# Nel codice
token := os.Getenv("GITHUB_TOKEN")
configURL := fmt.Sprintf("https://github.com/org/repo.git#config.json?auth=token:%s", token)
```

### Audit e Compliance

Il provider include logging automatico di eventi di sicurezza:

- Tentativi di path traversal
- Accessi a file sensibili
- Errori di autenticazione
- Rate limiting
- Operazioni fallite

## Supporto

### Repository
- **Source Code**: https://github.com/agilira/argus-provider-git
- **Issues**: https://github.com/agilira/argus-provider-git/issues
- **Documentazione**: https://github.com/agilira/argus-provider-git/blob/main/README.md

### Licenza
MPL-2.0 - Vedere LICENSE.md per dettagli completi.

### Contributi
I contributi sono benvenuti! Vedere CONTRIBUTING.md per linee guida.

---

**Nota**: Questa documentazione è per il provider Git di Argus v1.0+. Per versioni precedenti, consultare la documentazione specifica della versione.