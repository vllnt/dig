# UI Testing (BLOCKING for any UI change)

UI changes are verified in a **real browser or device** — never assumed to work
because the build passed.

## Triggers (ANY -> required)

- A UI file changed (`.tsx/.jsx/.vue/.svelte`, CSS/SCSS, layout/route files).
- A component/screen/page was added or modified.

## Two layers (both required)

1. **Persistent test** — a Playwright spec (web) or Maestro flow (mobile) that a
   user-like interaction drives and asserts an outcome on.
2. **Live visual check** — load the changed route in a real browser, confirm:
   - zero console errors, no hydration failures, no 4xx/5xx,
   - renders correctly at desktop / tablet / mobile viewports.

## Test user OUTCOMES, not existence

```
BAD:  expect(page.locator('.btn')).toBeVisible()
GOOD: await page.getByRole('button', { name: 'Save' }).click()
      await expect(page.getByText('Saved')).toBeVisible()
```

| Tool by platform | |
|------------------|---|
| Next/React/Vue/Svelte/Astro (web) | Playwright + a real browser |
| Expo Web | Playwright (it's a web app on localhost) |
| Expo/React Native (native) | Maestro on a real/simulated device |

## Anti-patterns (NEVER)

- "It's just a style tweak" -> still verify visually.
- Mocking the browser/DOM (jsdom is for unit logic, not UI verification).
- Asserting element existence only — assert the user can accomplish the goal.
