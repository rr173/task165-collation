package service
import ("context"; "testing")
func TestBug03_SnapshotViewIncludesFrozenEvidence(t *testing.T) {
 svc,_:=newTestService(t); ctx:=context.Background(); p,_:=svc.CreateProject(ctx,"证据视图",""); b,_:=svc.CreateWitness(ctx,p.ID,"B","底本","",true)
 if _,err:=svc.ImportPassages(ctx,b.ID,"证据正文。");err!=nil {t.Fatal(err)}; sn,err:=svc.BuildSnapshot(ctx,p.ID);if err!=nil{t.Fatal(err)}; v,err:=svc.GetSnapshot(ctx,sn.ID);if err!=nil{t.Fatal(err)};if len(v.PassageHashes)==0{t.Fatal("snapshot view omitted frozen evidence")}
}
