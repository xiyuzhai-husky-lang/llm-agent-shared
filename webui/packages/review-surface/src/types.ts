export interface ReviewDocumentSummary {
  id: string
  title: string
  path: string
  kind: string
}

export interface ReviewDocument extends ReviewDocumentSummary {
  body: string
}

export interface ReviewComment {
  id: string
  author: string
  timestamp: string
  body: string
}

export interface ReviewCommentThread {
  comments: ReviewComment[]
}

export interface NewReviewComment {
  author: string
  body: string
}

export type ReviewStatus = 'pending' | 'reviewed' | 'blocked'

export interface ReviewDocumentState {
  status: ReviewStatus
  updatedAt?: string
}
