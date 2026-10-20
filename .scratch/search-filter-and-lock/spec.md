Status: ready-for-agent

## Problem Statement

When an AI assistant inside LibreChat (or another MCP client) attempts to discover tools from a specific server by passing an explicit `servers` filter—such as `search_tools(query="search notes in second brain vault", servers=["second-brain"])`—the MCP bridge crashes with a runtime panic:
`runtime error: index out of range [498] with length 27` inside the fusion scoring algorithm.

This crash immediately terminates the server process. When the container orchestration system (Docker) attempts to restart the container, the bridge encounters a secondary failure during startup:
`mcp-bridge: state directory "/app/configs/data": storage: store already has a writer: /app/configs/data`
causing the bridge to exit with code 1 and enter an infinite crash loop.

Because the bridge is caught in a restart loop, all subsequent client connection attempts fail. LibreChat detects the repeated connection drops, trips its internal client circuit breaker (`[MCP][tools] Circuit breaker is open, connection attempt blocked`), and reports to the user that the tool bridge is down.

## Solution

1. **Server Filter Index Mapping in Hybrid Search**:
   Align the candidate document indexing between the global lexical index and the filtered document set. When an explicit server filter narrows the candidate catalog from the global corpus to a subset of eligible documents, map each document in the global corpus to its corresponding index within the eligible subset. Filter raw lexical search matches through this mapping, discarding matches from unselected servers and translating eligible document indices into the local coordinate space of the eligible subset before reciprocal rank fusion. This guarantees that all fusion operations access valid slice bounds and only rank tools belonging to the requested servers.

2. **Kernel Advisory Locking for Crash-Resilient State Storage**:
   Transition state store exclusivity from simple filesystem existence (`O_CREATE|O_EXCL`) to operating system advisory file locking (`flock`). Under advisory locking, the operating system kernel automatically releases the lock descriptor the moment a process terminates—regardless of whether it exited cleanly, encountered a panic, received a SIGKILL, or was killed by an OOM killer. When the container restarts, the new process can acquire the writer lock immediately without requiring manual filesystem intervention to delete stale lock files.

## User Stories

1. As an AI assistant in LibreChat, I want to narrow tool searches using the `servers` parameter, so that I find tools specifically scoped to the target system (e.g., `second-brain`).
2. As a second-brain user, I want tool discovery queries targeting my notes and knowledge base to execute without crashing the MCP bridge.
3. As an AI assistant, I want `search_tools` to remain stable and responsive regardless of whether the query matches tools in the selected server, unselected servers, or nowhere in the catalog.
4. As an AI assistant, I want subsequent tool discovery and tool execution calls to proceed without encountering open circuit breaker errors caused by backend server crashes.
5. As an operator, I want the hybrid search engine to accurately translate global lexical index matches to the filtered server's document set, so that slice index out-of-range panics cannot occur.
6. As an operator, I want lexical BM25 ranking to exclude documents from servers that were not requested in the active server filter.
7. As an operator, I want hybrid reciprocal rank fusion (RRF) to combine lexical and vector scores only across documents eligible under the active server filter.
8. As an operator, I want vector ranking and BM25 ranking to operate on the same consistent candidate document indices when a server filter is applied.
9. As an operator, I want the bridge container to automatically restart and become ready after an unexpected process crash without human intervention or manual lockfile deletion.
10. As an operator, I want the persistent state directory lock to be bound to the lifetime of the running process via kernel advisory locking, so that abnormal termination immediately frees the lock.
11. As an operator, I want a restarted container instance to automatically acquire the writer lock even if the previous container process terminated abruptly due to a panic, OOM, or SIGKILL.
12. As an operator, I want concurrent duplicate bridge instances to be prevented from simultaneously writing to the same state directory.
13. As an operator, I want clean shutdowns to continue releasing the writer lock and deleting the lock file from disk.
14. As an operator, I want the bridge to report clean HTTP 200 responses to `search_tools` when server filters match valid tools.
15. As an operator, I want the bridge to report valid JSON error responses instead of crashing if invalid server filters or malformed arguments are provided.
16. As a LibreChat user, I want to use discovered tools like `wiki_search_notes` directly after searching for them, without the discovery layer crashing mid-turn.
17. As an AI assistant, I want searches with no matching tools under a server filter to return an empty result list gracefully rather than panicking.
18. As an operator, I want existing search ranking behavior and weights for queries without server filters to remain completely unchanged.
19. As an operator, I want durable embeddings stored on disk to remain valid and loadable across process crashes and restarts.
20. As a developer, I want regression tests to verify that server-filtered searches across large multi-server catalogs execute cleanly and deterministically.
21. As a developer, I want regression tests to verify that stale lock files left by terminated processes do not block subsequent store initialization.

## Implementation Decisions

- **Index Alignment in Search Scoring**:
  The search module will maintain an index mapping between the global document snapshot and the filtered subset of eligible documents. When evaluating lexical search matches, any document belonging to a filtered-out server will be discarded, and all matching eligible documents will have their indices translated to the local candidate list. Reciprocal rank fusion will only receive candidate indices that are strictly bounded by the size of the eligible document list.

- **Candidate Scoping**:
  When no server filter is provided, the index mapping is an identity mapping covering the entire snapshot. When a server filter is provided, the mapping maps each global index to its position in the filtered slice (or indicates absence).

- **Kernel-Managed Process Locks**:
  The storage module will use non-blocking exclusive advisory file locks on the writer lock file. When opening the state store, the module will open the lock file and apply an exclusive non-blocking lock. If another live process holds the lock, the store initialization will return the existing writer locked error. When the process holding the lock exits (normally or abnormally), the operating system kernel closes the descriptor and drops the advisory lock, allowing a new process to acquire it without encountering stale lock file errors.

- **Lock Release and Cleanup**:
  During a clean shutdown, the storage module will release the advisory lock, close the file descriptor, and remove the lock file from the filesystem. If abnormal termination skips file removal, the next startup will safely acquire the advisory lock on the existing file descriptor.

- **Preservation of External Contracts**:
  The public tool schemas, search request/response structures, error codes, and configuration schemas remain unchanged.

## Testing Decisions

- **Good Test Standard**:
  Tests must assert external observable behavior:
  - Performing a search with a server filter on a catalog with multiple servers must return only the matching tools from the requested server, produce a valid response envelope, and not panic.
  - Initializing a state store when a stale lock file exists from an unowned process must succeed and permit read/write operations.
  - Attempting to initialize a second store concurrently on the same directory while the first store is active must fail with the writer locked error.
  Tests must not inspect private internal mapping arrays or file descriptor numbers.

- **Modules Tested**:
  1. **Search Engine**: Unit tests using synthetic multi-server snapshots where total catalog size significantly exceeds the filtered server size, with queries matching terms in both selected and unselected servers.
  2. **Storage Management**: Store tests verifying lock acquisition, lock collision rejection for active concurrent writers, and lock acquisition over pre-existing stale lock files.
  3. **End-to-End HTTP Wire (`cmd/mcp-bridge`)**: Integration tests driving `POST /mcp` with `search_tools` carrying server filters against real composition roots.

- **Prior Art**:
  - `internal/search/search_test.go`: `TestServerFilterNarrowsAndRejectsUnknown` (needs expansion to cover catalogs larger than eligible subsets).
  - `internal/storage/store_test.go`: `TestOpenCreatesPrivateLayoutAndExclusiveWriter`.

## Out of Scope

- Changing the underlying BM25 formula or tokenization rules.
- Modifying vector embedding dimensions, providers, or caching structures.
- Altering the on-disk JSON snapshot schema for tools or prompts.
- Modifying LibreChat's internal client circuit breaker parameters.

## Further Notes

- The user can immediately unblock their current crash-looping container by removing `.writer.lock` from the mounted persistent storage volume on the host. Once removed, the container will start up until a new server-filtered search triggers the panic.
- Implementing this spec permanently fixes both the search panic and the crash-recovery restart loop.
