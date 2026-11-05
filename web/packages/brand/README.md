# brand

LocalStack brand tokens, logos and typeface for the demo UIs. Every demo UI imports this package, so the demos look like LocalStack and not like a one-off.

Positioning: these demos are **LocalStack for SaaS Applications**.

## What is here

| Path | What it is |
|---|---|
| `tokens.css` | The five official colors, the dark background gradient, an error color, a warning color, and the Exo font face, as CSS custom properties (`--ls-*`) |
| `shadcn-theme.css` | The shadcn variable names (`--background`, `--primary` and the rest), set from the tokens, so components written against shadcn pick up the brand |
| `src/variants/` | `cva` variants for `Button` and `Badge`. Mint is the action color. |
| `logo/localstack-logo-horizontal-color.svg` | Gradient mark with a dark wordmark. Use on light backgrounds. |
| `logo/localstack-logo-horizontal-light.svg` | All light grey. Use on dark backgrounds. |
| `logo/localstack-logo-icon-color.svg` | The mark alone, for small spaces such as a favicon |
| `fonts/Exo-Variable.ttf`, `fonts/OFL.txt` | Exo, from `google/fonts` under the SIL Open Font License. Bundled so a demo works with no network. |

## Source

The LocalStack style guide and logo package, from the Branding page in Notion (Areas / Marketing / Branding). The logos are from `localstack-logo-package.zip`, unmodified.

## Rules, from the style guide

- Use only the colors in `tokens.css`. The style guide has no error or warning color, so `--ls-danger` and `--ls-warning` are additions. `--ls-danger` is for failures and declines only. `--ls-warning` is for a state that is not wrong but is not the usual case, such as production mode.
- Set text in Exo.
- On a dark background use the light logo. On a light background use the color logo. Do not recolor or redraw the logo.
- Keep the clear space shown on page 04 of the style guide (X/2 around the horizontal logo, X/5 around the vertical one).

## Not decided

The style guide has no lockup for a product line such as "LocalStack for SaaS Applications". Until the design owner decides one, set the product name as text in Exo beside or below the logo. Do not alter the logo to include it.
