// MongoDB index creation for ranking-service
// Run: mongosh cvedb migrations/001_ranking_indexes.js

db.ranking.createIndex(
  { "cpe": 1 },
  { unique: true, name: "ranking_cpe_unique" }
);

db.ranking.createIndex(
  { "rank.group": 1 },
  { name: "ranking_group" }
);

print("ranking-service indexes created successfully");
