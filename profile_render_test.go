package main

import (
 "bytes"
 "html/template"
 "io"
 "net/http"
 "strings"
 "testing"
)
func TestProfileRendersSavedBoardAndContinuation(t *testing.T) {
 tpl,err:=template.ParseFS(templatesFS,"templates/profile.html");if err!=nil{t.Fatal(err)}
 var out bytes.Buffer
 err=tpl.ExecuteTemplate(&out,"profile.html",map[string]interface{}{"Profile":Profile{Username:"example",FullName:"Example"},"Boards":[]Board{{ID:"1",URL:"/example/board/",Name:"Board",ImageURL:"/image?url=x"}},"ProfilePins":[]Pin{{ID:"123",ImageURL:"/image?url=y"}},"PinsBookmark":"next"})
 if err!=nil{t.Fatal(err)}
 for _,part:=range []string{"/example/board/","/pin/123","pins_cursor=next","data-profile-feed","</html>"}{if !strings.Contains(out.String(),part){t.Fatalf("missing %s",part)}}
}
type profileTransport func(*http.Request)(*http.Response,error)
func(f profileTransport)RoundTrip(r *http.Request)(*http.Response,error){return f(r)}
func TestSavedFeedsDistinguishFailureFromEmpty(t *testing.T){
 old:=http.DefaultTransport;defer func(){http.DefaultTransport=old}()
 for _,tc:=range []struct{status int;body string;failed bool}{{503,`{}`,true},{200,`{"resource_response":{"error":{"message":"unavailable"}}}`,true},{200,`{"resource_response":{"data":[]}}`,false}}{
 http.DefaultTransport=profileTransport(func(r *http.Request)(*http.Response,error){return &http.Response{StatusCode:tc.status,Header:make(http.Header),Body:io.NopCloser(strings.NewReader(tc.body))},nil})
 boards,_,_:=fetchBoards("example","","");if(boards==nil)!=tc.failed{t.Fatal("boards failure state")}
 pins,_:=fetchUserProfilePins("example","","");if(pins==nil)!=tc.failed{t.Fatal("pins failure state")}
 }
}
