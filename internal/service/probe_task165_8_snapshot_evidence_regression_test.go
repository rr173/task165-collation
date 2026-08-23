package service
import("context";"testing")
func TestBug08_SnapshotEvidenceSurvivesProjection(t *testing.T){svc,_:=newTestService(t);c:=context.Background();p,_:=svc.CreateProject(c,"证据存活","");b,_:=svc.CreateWitness(c,p.ID,"B","底本","",true);if _,e:=svc.ImportPassages(c,b.ID,"存活正文。");e!=nil{t.Fatal(e)};s,e:=svc.BuildSnapshot(c,p.ID);if e!=nil{t.Fatal(e)};v,e:=svc.GetSnapshot(c,s.ID);if e!=nil{t.Fatal(e)};if len(v.PassageHashes)==0{t.Fatal("frozen evidence disappeared")}}
