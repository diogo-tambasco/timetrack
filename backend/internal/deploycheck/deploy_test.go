// Teste de inspeção dos artefatos de deploy (CA-16, fase 00 T-16).
// Os artefatos NÃO são aplicados na VPS aqui — só validados.
package deploycheck

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]struct {
		Image   string   `yaml:"image"`
		Ports   []string `yaml:"ports"`
		Restart string   `yaml:"restart"`
	} `yaml:"services"`
	Networks map[string]struct {
		External bool `yaml:"external"`
	} `yaml:"networks"`
}

func TestDeployCompose_IsoladoDaVPS(t *testing.T) {
	raw, err := os.ReadFile("../../../deploy/docker-compose.yml")
	if err != nil {
		t.Fatalf("deploy/docker-compose.yml ausente: %v", err)
	}

	var c composeFile
	if err := yaml.Unmarshal(raw, &c); err != nil {
		t.Fatalf("YAML inválido: %v", err)
	}
	if len(c.Services) == 0 {
		t.Fatal("compose sem serviços")
	}

	for name, svc := range c.Services {
		// nunca tomar 80/443 da VPS (quem expõe é o Caddy existente)
		for _, p := range svc.Ports {
			host := strings.Split(p, ":")[0]
			if host == "80" || host == "443" {
				t.Errorf("serviço %s publica a porta proibida %s", name, p)
			}
		}
		if strings.Contains(strings.ToLower(svc.Image), "caddy") {
			t.Errorf("serviço %s declara um Caddy próprio (%s) — proibido", name, svc.Image)
		}
		if svc.Restart != "unless-stopped" {
			t.Errorf("serviço %s sem restart unless-stopped (veio %q)", name, svc.Restart)
		}
	}

	// o app não publica porta nenhuma em produção (D-10)
	if app, ok := c.Services["timetrack-app"]; !ok {
		t.Error("serviço timetrack-app ausente")
	} else if len(app.Ports) != 0 {
		t.Errorf("timetrack-app não deve publicar portas em produção, veio %v", app.Ports)
	}

	// rede do Caddy existente referenciada como external
	foundExternal := false
	for _, n := range c.Networks {
		if n.External {
			foundExternal = true
		}
	}
	if !foundExternal {
		t.Error("nenhuma rede marcada external: true (rede do Caddy existente)")
	}

	// vizinhos da VPS seguem proibidos no conteúdo OPERANTE — a única menção
	// sancionada é o NOME da rede external do Caddy existente,
	// primaos_primaos_net (fase 09 T-02: o vhost só alcança o app por essa
	// rede; em produção desde a sessão 10). Comentários ficam fora do scan
	// (documentam a decisão sem operar nada).
	var operante []string
	for _, line := range strings.Split(strings.ToLower(string(raw)), "\n") {
		if trim := strings.TrimSpace(line); !strings.HasPrefix(trim, "#") {
			operante = append(operante, line)
		}
	}
	semRedeSancionada := strings.ReplaceAll(strings.Join(operante, "\n"), "primaos_primaos_net", "")
	if strings.Contains(semRedeSancionada, "primaos") ||
		strings.Contains(semRedeSancionada, "jarvis") ||
		strings.Contains(semRedeSancionada, "brain") {
		t.Error("compose de deploy menciona vizinhos da VPS — proibido")
	}
}

func TestCaddySnippet_SoOVhost(t *testing.T) {
	raw, err := os.ReadFile("../../../deploy/Caddyfile.snippet")
	if err != nil {
		t.Fatalf("deploy/Caddyfile.snippet ausente: %v", err)
	}
	content := string(raw)

	// domínio REAL (Hostinger) — corrigido na sessão 9: timetrack.live nunca
	// foi do Diogo; só o module path Go o mantém como identificador interno
	if !strings.Contains(content, "timmingtrack.life") {
		t.Error("snippet sem o vhost timmingtrack.life")
	}
	if !strings.Contains(content, "reverse_proxy timetrack-app:8080") {
		t.Error("snippet sem reverse_proxy timetrack-app:8080")
	}

	// nada além do vhost: todo conteúdo não-comentário vive dentro do bloco
	depth := 0
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if depth == 0 && !strings.HasPrefix(trim, "timmingtrack.life") {
			t.Errorf("conteúdo fora do bloco do vhost: %q", trim)
		}
		depth += strings.Count(trim, "{") - strings.Count(trim, "}")
	}
	if depth != 0 {
		t.Error("chaves desbalanceadas no snippet")
	}
}

func TestDeployEnvExample_SoPlaceholders(t *testing.T) {
	raw, err := os.ReadFile("../../../deploy/.env.example")
	if err != nil {
		t.Fatalf("deploy/.env.example ausente: %v", err)
	}
	for _, v := range []string{"DATABASE_URL=", "POSTGRES_PASSWORD="} {
		if !strings.Contains(string(raw), v) {
			t.Errorf(".env.example de deploy sem %s", v)
		}
	}
}
