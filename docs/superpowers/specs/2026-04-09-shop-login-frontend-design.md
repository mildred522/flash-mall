# Flash Mall Shop And Login Frontend Design

## Objective

Upgrade the current demo-like frontend into a presentation-ready product surface while preserving the existing login, order, health-check, and debug flows.

The redesign focuses on:

- turning `/shop` into a believable ecommerce homepage
- reusing the visual language from the Figma `Shoppe` template
- moving engineering visibility into a developer floating console instead of exposing it in the main content flow
- keeping `/debug` as the full engineering control surface

## Scope

In scope:

- redesign `/shop`
- add an in-page login modal on `/shop`
- add a floating developer console on `/shop`
- lightly refresh `/` so it matches the new visual direction
- validate the resulting UI in browser with Chrome DevTools MCP

Out of scope:

- replacing backend APIs
- changing login semantics or order-creation semantics
- redesigning `/debug` into a consumer-facing surface
- adding a new frontend framework or build pipeline

## Design Direction

### Visual Thesis

Keep the airy blue-white identity from the Figma `Shoppe` mobile design, but reinterpret it as a desktop-friendly ecommerce homepage with stronger product credibility and cleaner responsive structure.

### Source Design

Figma file:

- `https://www.figma.com/design/r6CJ6lVl5yUnynNPxDl8fs/Shoppe---eCommerce-Clothing-Fashion-Store-Multi-Purpose-UI-Mobile-App-Design--Community-`

Primary node references:

- Login reference: `03 Login` (`0:12718`)
- Supporting visual language: rounded surfaces, blue organic background shapes, bold headline typography, soft neutral forms

### Product Positioning

The homepage should feel like a real consumer storefront first. Engineering affordances remain visible, but they live in a clearly separated developer console layer.

## Information Architecture

### `/shop`

`/shop` becomes the main presentation surface.

Sections:

1. Top navigation
2. Hero section
3. Campaign and discovery section
4. Featured products section
5. Lightweight footer/support area

The page must remain a functional demo entry point for:

- login
- order creation
- quick purchase flow
- live status visibility

### `/debug`

`/debug` stays intact as the full engineering workbench.

Role split:

- `/shop`: product-facing demonstration page
- `/debug`: engineering deep-dive page

### `/`

The landing page gets a lightweight refresh so it visually aligns with the redesigned `/shop`, while preserving the current navigation purpose:

- enter shop
- enter debug console

## `/shop` Layout Design

### Hero

The first viewport should look like a polished ecommerce homepage, not a control panel.

Hero content:

- strong product or brand statement
- one primary CTA to shop
- one secondary CTA to open login
- one campaign or drop indicator
- one product-oriented visual cluster on the right side

Desktop behavior:

- two-column composition
- left column for messaging and CTA
- right column for visual merchandising and highlight blocks

Mobile behavior:

- stacked layout
- hero visual reduced in height
- CTA buttons remain visible without excessive scroll

### Campaign And Discovery

This area should expose the flash-sale nature of the project without turning into a diagnostic dashboard.

Content types:

- flash sale countdown
- category shortcuts
- featured collection blocks
- promotional highlight cards

### Featured Products

Products remain interactive and tied to the existing backend order endpoint.

Each featured item should support:

- name
- short merchandising description
- price or promotional label
- quantity selection
- buy action

The product area should feel like merchandised storefront content, not a bare CRUD list.

## Login Experience

### Interaction Model

Login is not a separate route. It is an in-page modal opened from `/shop`.

Reasons:

- matches the goal of keeping the homepage as the main experience
- reduces route switching during demonstrations
- keeps the Figma login language as a focused moment inside the storefront

### Visual Model

The modal adapts the Figma `03 Login` screen:

- large `Login` headline
- blue bubble-like background motifs
- soft rounded input fields
- prominent blue primary button
- light, premium composition

### Functional Model

The modal uses the current backend login endpoint:

- `POST /api/auth/login`

Fields stay aligned with current backend reality:

- `user_id`
- `password`

Successful login should:

- close the modal
- update the top navigation account state
- update the developer console auth status
- preserve token in the same local storage path the current UI uses

Failed login should:

- show clear inline error feedback
- log the attempt into the developer console

## Developer Floating Console

### Purpose

Preserve the project's engineering story without degrading the main storefront aesthetic.

### Placement And Responsive Rules

Desktop:

- fixed bottom-right
- expanded by default

Mobile:

- collapsed by default
- opened via a floating action button

### Visual Style

The console should contrast the storefront:

- darker surface
- slightly translucent or layered feel
- strong separation from main content
- compact but readable typography

It should feel like a controlled developer overlay, not a random widget.

### Console Sections

Required sections:

1. Auth
2. Health
3. Quick actions
4. Logs

Auth section:

- logged-in / logged-out state
- user ID
- token preview
- login and logout actions

Health section:

- trigger health check
- show key dependency states

Quick actions:

- quick buy
- idempotency-oriented action entry point
- optional rapid purchase action if it fits compactly

Logs:

- request and response summary
- order result
- failure detail
- recent activity stream

## Existing Behavior Preservation

The redesign must preserve the current operational capabilities:

- login through the existing login API
- order creation through the existing order API
- token persistence through local storage
- health checks through the current health endpoint
- visible success and error states

No backend contract changes are part of this work.

## Technical Translation

### Implementation Surface

The current frontend is server-served embedded HTML in:

- `app/order/api/internal/handler/web/shop.html`
- `app/order/api/internal/handler/web/home.html`
- `app/order/api/internal/handler/web/debug.html`

The redesign should follow the current project delivery model:

- embedded HTML
- inline CSS and JS where already established
- no new frontend build system

### Figma Translation Rules

The Figma file is mobile-first. The implementation will not mechanically reproduce a 375px mobile artboard inside desktop width.

Instead it will preserve:

- color language
- typography hierarchy
- rounded controls
- background motif style
- emotional tone

while translating:

- composition into desktop-responsive layouts
- forms into project-compatible HTML
- action areas into current API flows

## Validation Plan

### Visual Validation

Use Figma MCP as the visual source of truth for the login language and overall brand direction.

Check:

- heading hierarchy
- color fidelity
- rounded form and button treatment
- visual balance between hero, product area, and overlay console

### Browser Validation

Use Chrome DevTools MCP to verify:

- desktop and mobile layouts
- login flow
- order flow
- developer console interactions
- absence of blocking console errors

### Acceptance Criteria

The work is complete when:

1. `/shop` reads as a believable ecommerce homepage
2. login happens through an in-page modal using the current backend
3. the floating developer console exposes auth, health, quick actions, and logs
4. `/debug` remains available as the full engineering page
5. `/` visually aligns with the new direction
6. the final page works on desktop and mobile

## Risks And Mitigations

### Risk: Mobile template translated too literally

Mitigation:

- preserve visual language, not strict mobile composition

### Risk: Engineering controls overwhelm storefront

Mitigation:

- isolate controls in the floating console
- keep main content commerce-first

### Risk: Demo utility is lost during polish

Mitigation:

- preserve all existing login, order, and health behaviors
- keep `/debug` untouched as the fallback engineering view

## Recommended Implementation Order

1. Refresh `/shop` structure and styling shell
2. Build login modal using current auth flow
3. Build floating developer console
4. Rewire product purchase interactions into the new homepage
5. Lightly align `/` to the new visual language
6. Validate in browser against both functionality and visual targets
