# PRD: Routing Refactoring - Unified Model Routing

## Introduction

현재 AmpCode의 모델 라우팅 로직이 여러 곳에 파편화되어 있습니다:
- `FallbackHandler` - 로컬/프록시 결정 + 모델 매핑
- `routing.Router` - 모델 매핑 + OAuth alias (미사용)
- `util.GetProviderName` - 모델에서 프로바이더 추론
- `DefaultModelMapper` - 모델 매핑 + fallback

이 리팩토링은 `internal/routing.Router`를 **단일 라우팅 결정 원천**으로 통합하고, 기존 동작을 100% 유지하면서 내부 구조를 개선합니다.

## Goals

- `Router`를 모델 라우팅 결정의 단일 원천으로 통합
- 모든 model-requesting 라우트에 통합 래퍼 적용
- Passthrough 라우트는 변경 없이 유지
- TDD 방식으로 기존 동작을 먼저 테스트로 기록
- 외부 동작 100% 호환성 유지

## Route Classification

### Type 1: Model-Requesting (라우팅 필요)
- `POST /api/provider/:provider/v1/chat/completions`
- `POST /api/provider/:provider/v1/messages`
- `POST /api/provider/:provider/v1/responses`
- `POST /api/provider/:provider/v1beta/models/*action`

### Type 2: Passthrough (항상 프록시)
- `/api/auth/*`, `/api/user/*`, `/api/threads/*`, `/api/telemetry/*`
- 관리/인증 관련 모든 경로

### Type 3: Mixed (조건부)
- `ANY /api/provider/google/v1beta1/*path` (POST + /models/ → 로컬, 그 외 → 프록시)

## User Stories

### US-001: Characterization 테스트 하네스 구축
**Description:** 개발자로서, 기존 라우팅 동작을 테스트로 기록하여 리팩토링 중 회귀를 방지하고 싶습니다.

**Acceptance Criteria:**
- [ ] httptest + gin.Engine 테스트 환경 구축
- [ ] Fake proxy recorder 구현 (호출 여부/요청 내용 기록)
- [ ] Fake local handler recorder 구현 (body/headers/context keys 기록)
- [ ] OpenAI `/chat/completions` 라우트 테스트 작성 (LOCAL_PROVIDER 경로)
- [ ] MODEL_MAPPING 경로 테스트 작성
- [ ] AMP_CREDITS 프록시 fallback 테스트 작성
- [ ] `go test ./internal/api/modules/amp/...` 통과

### US-002: Mixed Route Gating 테스트
**Description:** 개발자로서, Gemini v1beta1 브릿지 라우트의 조건부 분기를 테스트로 기록하고 싶습니다.

**Acceptance Criteria:**
- [ ] POST + /models/ 포함 시 로컬 핸들러 호출 테스트
- [ ] POST + /models/ 미포함 시 프록시 호출 테스트
- [ ] GET 요청 시 항상 프록시 호출 테스트
- [ ] `go test ./internal/api/modules/amp/...` 통과

### US-003: ModelExtractor 인터페이스 추출
**Description:** 개발자로서, 모델 추출 로직을 순수 함수로 분리하여 테스트 가능하게 만들고 싶습니다.

**Acceptance Criteria:**
- [ ] `ModelExtractor` 인터페이스 정의
- [ ] JSON body에서 모델 추출 테스트
- [ ] Gemini v1beta action 파라미터에서 모델 추출 테스트
- [ ] Gemini v1beta1 path에서 모델 추출 테스트
- [ ] 모델 없을 때 빈 문자열 반환 테스트
- [ ] 기존 동작 변경 없음 (characterization 테스트 통과)

### US-004: ModelRewriter 인터페이스 추출
**Description:** 개발자로서, 요청/응답 모델 리라이트 로직을 분리하여 테스트 가능하게 만들고 싶습니다.

**Acceptance Criteria:**
- [ ] `ModelRewriter` 인터페이스 정의
- [ ] 요청 body 모델 리라이트 테스트
- [ ] 응답 writer 래핑 테스트
- [ ] 기존 동작 변경 없음 (characterization 테스트 통과)

### US-005: Router 계약 정의 및 단위 테스트
**Description:** 개발자로서, Router의 라우팅 결정 로직을 명확한 계약으로 정의하고 단위 테스트하고 싶습니다.

**Acceptance Criteria:**
- [ ] `RoutingRequest` / `RoutingDecision` 타입 정의
- [ ] `PreferLocalProvider` (default mode) 동작 테스트
- [ ] `ForceModelMapping` (force mode) 동작 테스트
- [ ] Thinking suffix 보존 테스트
- [ ] Fallback 모델 리스트 생성 테스트
- [ ] 기존 동작 변경 없음

### US-006: ModelRoutingWrapper 구현
**Description:** 개발자로서, 통합 라우팅 래퍼를 구현하여 FallbackHandler를 대체하고 싶습니다.

**Acceptance Criteria:**
- [ ] `ModelRoutingWrapper` 타입 구현
- [ ] Router 결정에 따른 로컬/프록시 분기
- [ ] MODEL_MAPPING 시 요청 body 리라이트
- [ ] MODEL_MAPPING 시 응답 모델 리라이트
- [ ] `Anthropic-Beta` 헤더 필터링 (로컬 경로만)
- [ ] Context keys 설정 (`mapped_model`, `fallback_models`)
- [ ] 하나의 라우트에 적용 후 characterization 테스트 통과

### US-007: 라우트별 점진적 마이그레이션
**Description:** 개발자로서, 모든 model-requesting 라우트를 새 래퍼로 마이그레이션하고 싶습니다.

**Acceptance Criteria:**
- [ ] OpenAI 라우트 마이그레이션
- [ ] Claude 라우트 마이그레이션
- [ ] Gemini v1beta 라우트 마이그레이션
- [ ] Gemini v1beta1 브릿지 마이그레이션
- [ ] 전체 characterization 테스트 통과
- [ ] `go build ./...` 통과
- [ ] `go test ./...` 통과

### US-008: 레거시 코드 정리
**Description:** 개발자로서, 중복된 레거시 라우팅 로직을 제거하고 싶습니다.

**Acceptance Criteria:**
- [ ] FallbackHandler의 중복 로직 제거 또는 thin adapter로 축소
- [ ] 사용되지 않는 함수 제거
- [ ] 전체 테스트 통과
- [ ] Dead code 없음 확인

## Functional Requirements

- FR-1: `Router.Resolve()`는 `RoutingDecision`을 반환해야 함
- FR-2: `RoutingDecision`은 `RouteType` (LOCAL_PROVIDER | MODEL_MAPPING | AMP_CREDITS | NO_PROVIDER) 포함
- FR-3: Thinking suffix는 매핑된 모델에도 보존되어야 함
- FR-4: Passthrough 라우트는 라우팅 로직을 거치지 않아야 함
- FR-5: 프록시 fallback은 로컬 후보가 0개일 때만 발생
- FR-6: `Anthropic-Beta` 필터링은 로컬 실행 경로에서만 적용
- FR-7: 응답 모델 리라이트는 MODEL_MAPPING 경로에서만 적용

## Non-Goals

- 외부 API 동작 변경
- 새로운 라우팅 기능 추가
- 설정 형식 변경
- 프로바이더 우선순위 로직 변경

## Technical Considerations

- `internal/routing/router.go` 확장 (현재 미사용 상태)
- `FallbackHandler`와 `Router`의 로직 통합
- 기존 `util.GetProviderName` 호환성 유지 (점진적 교체)
- Body 읽기/복원 로직 유지 (스트리밍 호환성)

## Success Metrics

- 모든 characterization 테스트 통과
- `go build ./...` 성공
- `go test ./...` 성공
- 코드 중복 제거 (FallbackHandler + Router 로직 통합)

## Open Questions

- `util.GetProviderName`을 완전히 registry 기반으로 교체할지, 호환성 레이어로 유지할지?
- 향후 멀티 프로바이더 retry 로직을 Executor에 추가할지?

## Branch Name

`refactor/unified-routing`
