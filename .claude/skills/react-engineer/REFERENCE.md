# React Reference

## Component structure

Keep components small and single-purpose. Split when a component does more than one thing or its JSX exceeds ~80 lines.

Co-locate files by feature:

```
features/
  deployments/
    DeploymentList.tsx
    DeploymentCard.tsx
    DeploymentForm.tsx
    deployments.queries.ts   # TanStack Query definitions
    deployments.store.ts     # Zustand slice (if needed)
    deployments.types.ts
```

Avoid grouping by type (`components/`, `hooks/`, `utils/` at the root) — it scatters related code across the tree.

## TanStack Query

Define queries and mutations in a dedicated `*.queries.ts` file, not inline in components.

```ts
// deployments.queries.ts
export const deploymentsQuery = () =>
  queryOptions({
    queryKey: ["deployments"],
    queryFn: () => fetch("/api/deployments").then((r) => r.json()),
  });

export const useDeleteDeployment = () =>
  useMutation({
    mutationFn: (id: string) =>
      fetch(`/api/deployments/${id}`, { method: "DELETE" }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["deployments"] }),
  });
```

Always handle `isPending`, `isError`, and `error` in the component that consumes a query or mutation. Never leave the user staring at a blank screen.

Invalidate related queries on successful mutations — don't manually update cache unless performance demands it.

## Zustand

Use Zustand only for state that is genuinely global and not derived from server data:

- UI state shared across distant components (e.g. selected deployment id, sidebar open)
- Ephemeral state that doesn't belong to the server (e.g. active filters, form draft)

Do not put server data in Zustand — that's TanStack Query's job.

Keep stores flat and small. One slice per feature domain.

```ts
// deployments.store.ts
interface DeploymentsStore {
  selectedId: string | null;
  setSelectedId: (id: string | null) => void;
}

export const useDeploymentsStore = create<DeploymentsStore>((set) => ({
  selectedId: null,
  setSelectedId: (id) => set({ selectedId: id }),
}));
```

## shadcn/ui

Always reach for shadcn/ui primitives before building custom components:

- `Button`, `Input`, `Select`, `Dialog`, `Sheet`, `Badge`, `Skeleton`, `Table`, `Form`
- Use `Form` + `react-hook-form` for any user-input form

Do not override shadcn styles with arbitrary CSS. Use Tailwind utilities via the `className` prop.

When a shadcn component doesn't exist, build a simple Tailwind component — not a new library.

## Tailwind

Use Tailwind utility classes directly on elements. Avoid long repetitive class strings — extract a component instead.

```tsx
// Bad: duplicated classes across 10 card instances
<div className="rounded-lg border bg-card p-4 shadow-sm">

// Good: extract once
function Card({ children }: { children: React.ReactNode }) {
  return <div className="rounded-lg border bg-card p-4 shadow-sm">{children}</div>;
}
```

Avoid arbitrary values (`w-[347px]`) unless there is a strong reason. Prefer the design-system scale.

## Data fetching patterns

The backend is the single source of truth. The GUI is stateless — never treat local state as authoritative when the server has the answer.

After a create/delete/update operation, invalidate the relevant query. The list will re-fetch and reflect the new state automatically.

```tsx
const { data: deployments, isPending, isError } = useQuery(deploymentsQuery());

if (isPending) return <DeploymentListSkeleton />;
if (isError) return <ErrorMessage />;
```

## Antipatterns to avoid

- `useEffect` for data fetching — use TanStack Query
- Server data in Zustand — creates stale data bugs
- Prop drilling more than 2 levels — lift to Zustand or query cache
- One massive component per page — split by responsibility
- `useState` for async server state — that's a query
- Ignoring loading/error states — always render feedback
- Hardcoded strings for API URLs — use a central `api.ts` constants file
