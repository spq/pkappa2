export interface Status {
    ImportJobCount: number,
    IndexCount: number,
    IndexLockCount: number,
    PcapCount: number,
    StreamCount: number,
    PacketCount: number,
    MergeJobRunning: boolean,
    TaggingJobRunning: boolean,
}
