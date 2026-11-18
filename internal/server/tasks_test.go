package server_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roqianjas/macpanel/internal/tasks"
	"github.com/roqianjas/macpanel/internal/testenv"
)

func TestValidCmdTable(t *testing.T) {
	o := testenv.NewP3Env(t)
	os.WriteFile(filepath.Join(o.Home, "Developer/proj/artisan"), []byte("<?php"), 0o644)
	good := []string{"php artisan queue:work --tries=3 --sleep=3", "node worker.js", "python3 -m celery worker", "npm run worker", "pnpm start", "composer run-script x", "php " + o.Home + "/Developer/proj/artisan schedule:run"}
	for _, c := range good {
		if err := tasks.ValidCmd(o.App, c); err != nil {
			t.Errorf("harus sah: %q: %v", c, err)
		}
	}
	bad := []string{
		"", "bash -c x", "sh script.sh", "curl http://x", "rm -rf x", "/bin/sh x", "./run.sh", "php;id", "php a && id", "php a | cat", "php $(id)", "php `id`", "php 'a b'", `php "a"`, "php a\\b", "php a\nid", "php a > x", "php a & b",
		"php -r phpinfo();", "php -R x", "php -B x", "php -d auto_prepend_file=/x a.php", "php -S 0.0.0.0:8000", "php -a", "php -f x.php", "node -e 1", "node --eval=1", "node -p 1", "node --print 1", "node --require /x", "node --inspect=0.0.0.0 x.js", "node --inspect-brk x.js",
		"python3 -c 1", "python -i", "npm exec x", "npm x", "pnpm dlx x", "pnpm exec x", "npm install x", "npx something", "composer exec x", "composer install", "uv run x",
		"php ../../etc/x.php", "php /etc/passwd", "php ~/x.php", "php /Users/someuser/.ssh/id_x", "php " + o.Home + "/outside/x.php", strings.Repeat("a", 301),
	}
	for _, c := range bad {
		if err := tasks.ValidCmd(o.App, c); err == nil {
			t.Errorf("harus DITOLAK: %q", c)
		}
	}
}
