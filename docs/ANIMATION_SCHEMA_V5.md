# Animation schema v5

Schema v5 is the semantic animation format for the v2 runtime path. Runtime code must ask for intents such as `left_click`, `drag_hold`, or `locomotion_fast`; it must not branch on concrete clip names such as `happy`, `cry`, `walk`, or `run`.

## Top-level fields

- `schema`: must be `5`.
- `id`: pet or compiled definition id.
- `name`: optional display name; defaults to `id` in the compiler.
- `scale`: optional display scale; defaults to `1` in the v5 compiler.
- `frame_width`, `frame_height`: required positive frame size.
- `animation_dir`: directory containing sprites.
- `default_animation`: required fallback clip id.
- `tags`: all semantic tags known to this definition.
- `animations`: concrete clip inventory.
- `intents`: intent fallback profiles.

## Animation clip fields

Each clip must declare `file`, `fps`, `frames`, and `tags`. Optional fields include `loop`, `duration_ms`, `priority`, `speed_px_s`, and `native_facing`.

Tags describe capability, not behavior logic. For example, a clip named `angry` may carry `emotion`, `negative`, `held`, `protest`, `stationary`; runtime only sees those tags through the resolver.

## Intent fallback fields

Each intent has `fallbacks`, each fallback containing:

- `required`: tags that must all be present.
- `preferred`: tags that add score.
- `excluded`: tags that disqualify a candidate.
- `preferred_bonus`: optional per matching preferred tag.
- `base_score`: optional group score offset.

Fallbacks are compiled once at load time. Runtime resolution uses precompiled candidate groups, cooldown state, and recent-history penalty.

## Migration note from schema v4

Schema v4 `interactions` map exact user actions to exact animation names. Schema v5 removes that coupling. Migrate by assigning semantic tags to clips and writing intent fallback groups. Temporary bridges may infer tags from legacy metadata, but final runtime integration should load v5 definitions directly.
