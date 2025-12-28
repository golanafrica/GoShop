#!/bin/sh
# wait-for-db.sh
#!/bin/sh
# wait-for-db.sh

set -e

host="$1"
port="$2"
shift 2
cmd="$@"

# Attendre que le port soit ouvert
until nc -z "$host" "$port"; do
  echo "⏳ En attente de PostgreSQL ($host:$port)..."
  sleep 2
done

# Attendre que la base soit prête à accepter des connexions
until PGPASSWORD="$DB_PASSWORD" pg_isready -h "$host" -p "$port" -U "$DB_USER" -d "$DB_NAME"; do
  echo "⏳ En attente que la base soit prête..."
  sleep 2
done

echo "✅ PostgreSQL prêt. Démarrage de l'application."
exec $cmd